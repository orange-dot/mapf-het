/**
 * @file ekk_consensus.c
 * @brief EK-KOR v2 - Threshold-Based Distributed Consensus Implementation
 *
 * @copyright Copyright (c) 2026 Elektrokombinacija
 *
 * NOVELTY: Threshold Consensus with Mutual Inhibition
 * - Density-dependent threshold voting
 * - Supermajority support for safety-critical decisions
 * - Mutual inhibition for competing proposals
 */

#include "ekk/ekk_consensus.h"
#include "ekk/ekk_hal.h"
#include <string.h>

/* ============================================================================
 * PRIVATE STATE
 * ============================================================================ */

/** Decision callback (application decides how to vote) */
static ekk_consensus_decide_cb g_decide_callback = NULL;

/** Completion callback (application notified of results) */
static ekk_consensus_complete_cb g_complete_callback = NULL;

/* ============================================================================
 * PRIVATE HELPERS
 * ============================================================================ */

/**
 * @brief Find ballot by ID
 * @return Index if found, -1 otherwise
 */
static int find_ballot_index(const ekk_consensus_t *cons, ekk_ballot_id_t id)
{
    for (uint32_t i = 0; i < cons->active_ballot_count; i++) {
        if (cons->ballots[i].id == id) {
            return (int)i;
        }
    }
    return -1;
}

/**
 * @brief Check if a ballot is inhibited
 */
static bool is_inhibited(const ekk_consensus_t *cons, ekk_ballot_id_t ballot_id,
                          ekk_time_us_t now)
{
    for (uint32_t i = 0; i < cons->inhibit_count; i++) {
        if (cons->inhibited[i] == ballot_id && cons->inhibit_until[i] > now) {
            return true;
        }
    }
    return false;
}

/**
 * @brief Allocate a free ballot slot
 * @return Index of free slot, -1 if none available
 */
static int allocate_ballot_slot(ekk_consensus_t *cons)
{
    if (cons->active_ballot_count >= EKK_MAX_BALLOTS) {
        return -1;
    }

    return (int)cons->active_ballot_count;
}

/**
 * @brief Evaluate ballot result based on votes and threshold
 */
static ekk_vote_result_t evaluate_ballot(const ekk_ballot_t *ballot,
                                          uint32_t neighbor_count)
{
    if (ballot->completed) {
        return ballot->result;
    }

    /* Check for inhibition (handled in tick) */
    /* Here we just evaluate vote counts */

    uint32_t total_votes = ballot->vote_count;
    uint32_t yes_votes = ballot->yes_count;
    uint32_t no_votes = ballot->no_count;

    /* If not all votes received, still pending */
    if (total_votes < neighbor_count) {
        /* However, check if approval is already impossible */
        uint32_t remaining = neighbor_count - total_votes;

        /* Best case: all remaining vote yes */
        uint32_t max_yes = yes_votes + remaining;
        ekk_fixed_t max_ratio = (neighbor_count > 0) ?
            (ekk_fixed_t)(((int64_t)max_yes << 16) / neighbor_count) : 0;

        if (max_ratio < ballot->threshold) {
            /* Even if all remaining vote yes, cannot reach threshold */
            return EKK_VOTE_REJECTED;
        }

        /* Check if already reached threshold */
        ekk_fixed_t current_ratio = (neighbor_count > 0) ?
            (ekk_fixed_t)(((int64_t)yes_votes << 16) / neighbor_count) : 0;

        if (current_ratio >= ballot->threshold) {
            /* Threshold reached! */
            return EKK_VOTE_APPROVED;
        }

        /* Still pending */
        return EKK_VOTE_PENDING;
    }

    /* All votes received, compute final result */
    ekk_fixed_t approval_ratio = (neighbor_count > 0) ?
        (ekk_fixed_t)(((int64_t)yes_votes << 16) / neighbor_count) : 0;

    if (approval_ratio >= ballot->threshold) {
        return EKK_VOTE_APPROVED;
    } else {
        return EKK_VOTE_REJECTED;
    }
}

/**
 * @brief Finalize a ballot with a result
 */
static void finalize_ballot(ekk_consensus_t *cons, ekk_ballot_t *ballot,
                             ekk_vote_result_t result)
{
    ballot->result = result;
    ballot->completed = true;

    /* Invoke completion callback */
    if (g_complete_callback != NULL) {
        g_complete_callback(cons, ballot, result);
    }
}

/**
 * @brief Remove completed ballots from active list
 */
static void cleanup_completed_ballots(ekk_consensus_t *cons)
{
    uint32_t write_idx = 0;

    for (uint32_t read_idx = 0; read_idx < cons->active_ballot_count; read_idx++) {
        if (!cons->ballots[read_idx].completed) {
            if (write_idx != read_idx) {
                cons->ballots[write_idx] = cons->ballots[read_idx];
            }
            write_idx++;
        }
    }

    cons->active_ballot_count = write_idx;
}

/**
 * @brief Broadcast proposal to neighbors
 */
static ekk_error_t broadcast_proposal(const ekk_consensus_t *cons,
                                       const ekk_ballot_t *ballot)
{
    ekk_proposal_msg_t msg = {
        .msg_type = EKK_MSG_PROPOSAL,
        .proposer_id = cons->my_id,
        .ballot_id = ballot->id,
        .type = ballot->type,
        .data = ballot->proposal_data,
        .threshold = ballot->threshold,
    };

    return ekk_hal_broadcast(EKK_MSG_PROPOSAL, &msg, sizeof(msg));
}

/**
 * @brief Send vote to proposer
 */
static ekk_error_t send_vote(const ekk_consensus_t *cons,
                              ekk_module_id_t proposer_id,
                              ekk_ballot_id_t ballot_id,
                              ekk_vote_value_t vote)
{
    ekk_vote_msg_t msg = {
        .msg_type = EKK_MSG_VOTE,
        .voter_id = cons->my_id,
        .ballot_id = ballot_id,
        .vote = vote,
        .timestamp = (uint32_t)(ekk_hal_time_us() & 0xFFFFFFFF),
    };

    return ekk_hal_send(proposer_id, EKK_MSG_VOTE, &msg, sizeof(msg));
}

/* ============================================================================
 * INITIALIZATION
 * ============================================================================ */

ekk_error_t ekk_consensus_init(ekk_consensus_t *cons,
                                ekk_module_id_t my_id,
                                const ekk_consensus_config_t *config)
{
    if (cons == NULL || my_id == EKK_INVALID_MODULE_ID) {
        return EKK_ERR_INVALID_ARG;
    }

    memset(cons, 0, sizeof(ekk_consensus_t));
    cons->my_id = my_id;
    cons->next_ballot_id = 1;  /* 0 is invalid */

    /* Apply configuration */
    if (config != NULL) {
        cons->config = *config;
    } else {
        ekk_consensus_config_t default_config = EKK_CONSENSUS_CONFIG_DEFAULT;
        cons->config = default_config;
    }

    return EKK_OK;
}

/* ============================================================================
 * PROPOSAL CREATION
 * ============================================================================ */

ekk_error_t ekk_consensus_propose(ekk_consensus_t *cons,
                                   ekk_proposal_type_t type,
                                   uint32_t data,
                                   ekk_fixed_t threshold,
                                   ekk_ballot_id_t *ballot_id)
{
    if (cons == NULL || ballot_id == NULL) {
        return EKK_ERR_INVALID_ARG;
    }

    /* Allocate ballot slot */
    int idx = allocate_ballot_slot(cons);
    if (idx < 0) {
        return EKK_ERR_BUSY;
    }

    ekk_time_us_t now = ekk_hal_time_us();

    /* Initialize ballot */
    ekk_ballot_t *ballot = &cons->ballots[idx];
    memset(ballot, 0, sizeof(ekk_ballot_t));

    ballot->id = cons->next_ballot_id++;
    ballot->type = type;
    ballot->proposer = cons->my_id;
    ballot->proposal_data = data;
    ballot->threshold = threshold;
    ballot->deadline = now + cons->config.vote_timeout;
    ballot->result = EKK_VOTE_PENDING;
    ballot->completed = false;

    /* Initialize vote tracking */
    memset(ballot->votes, EKK_VOTE_ABSTAIN, sizeof(ballot->votes));

    /* Self-vote if allowed */
    if (cons->config.allow_self_vote) {
        /* Proposer implicitly votes yes for own proposal */
        ballot->votes[0] = EKK_VOTE_YES;
        ballot->vote_count = 1;
        ballot->yes_count = 1;
    }

    cons->active_ballot_count++;

    /* Broadcast proposal */
    broadcast_proposal(cons, ballot);

    *ballot_id = ballot->id;
    return EKK_OK;
}

/* ============================================================================
 * VOTING
 * ============================================================================ */

ekk_error_t ekk_consensus_vote(ekk_consensus_t *cons,
                                ekk_ballot_id_t ballot_id,
                                ekk_vote_value_t vote)
{
    if (cons == NULL || ballot_id == EKK_INVALID_BALLOT_ID) {
        return EKK_ERR_INVALID_ARG;
    }

    /* Find ballot */
    int idx = find_ballot_index(cons, ballot_id);
    if (idx < 0) {
        return EKK_ERR_NOT_FOUND;
    }

    ekk_ballot_t *ballot = &cons->ballots[idx];

    /* Cannot vote on completed ballot */
    if (ballot->completed) {
        return EKK_ERR_BUSY;
    }

    /* Send vote to proposer */
    return send_vote(cons, ballot->proposer, ballot_id, vote);
}

/* ============================================================================
 * INHIBITION
 * ============================================================================ */

ekk_error_t ekk_consensus_inhibit(ekk_consensus_t *cons,
                                   ekk_ballot_id_t ballot_id)
{
    if (cons == NULL || ballot_id == EKK_INVALID_BALLOT_ID) {
        return EKK_ERR_INVALID_ARG;
    }

    /* Check if already inhibited */
    ekk_time_us_t now = ekk_hal_time_us();
    for (uint32_t i = 0; i < cons->inhibit_count; i++) {
        if (cons->inhibited[i] == ballot_id) {
            /* Update expiry */
            cons->inhibit_until[i] = now + cons->config.inhibit_duration;
            return EKK_OK;
        }
    }

    /* Add new inhibition */
    if (cons->inhibit_count >= EKK_MAX_BALLOTS) {
        /* Evict oldest */
        for (uint32_t i = 0; i < cons->inhibit_count - 1; i++) {
            cons->inhibited[i] = cons->inhibited[i + 1];
            cons->inhibit_until[i] = cons->inhibit_until[i + 1];
        }
        cons->inhibit_count--;
    }

    cons->inhibited[cons->inhibit_count] = ballot_id;
    cons->inhibit_until[cons->inhibit_count] = now + cons->config.inhibit_duration;
    cons->inhibit_count++;

    /* Mark local ballot as cancelled if we have it */
    int idx = find_ballot_index(cons, ballot_id);
    if (idx >= 0) {
        finalize_ballot(cons, &cons->ballots[idx], EKK_VOTE_CANCELLED);
    }

    /* Broadcast inhibit message */
    /* Note: Using vote message with INHIBIT value */
    send_vote(cons, EKK_BROADCAST_ID, ballot_id, EKK_VOTE_INHIBIT);

    return EKK_OK;
}

/* ============================================================================
 * INCOMING MESSAGE HANDLERS
 * ============================================================================ */

ekk_error_t ekk_consensus_on_vote(ekk_consensus_t *cons,
                                   ekk_module_id_t voter_id,
                                   ekk_ballot_id_t ballot_id,
                                   ekk_vote_value_t vote)
{
    if (cons == NULL || voter_id == EKK_INVALID_MODULE_ID ||
        ballot_id == EKK_INVALID_BALLOT_ID) {
        return EKK_ERR_INVALID_ARG;
    }

    /* Handle inhibition */
    if (vote == EKK_VOTE_INHIBIT) {
        ekk_time_us_t now = ekk_hal_time_us();

        /* Add to inhibit list */
        if (cons->inhibit_count < EKK_MAX_BALLOTS) {
            cons->inhibited[cons->inhibit_count] = ballot_id;
            cons->inhibit_until[cons->inhibit_count] = now + cons->config.inhibit_duration;
            cons->inhibit_count++;
        }

        /* Cancel local ballot */
        int idx = find_ballot_index(cons, ballot_id);
        if (idx >= 0 && !cons->ballots[idx].completed) {
            finalize_ballot(cons, &cons->ballots[idx], EKK_VOTE_CANCELLED);
        }

        return EKK_OK;
    }

    /* Find ballot */
    int idx = find_ballot_index(cons, ballot_id);
    if (idx < 0) {
        /* Unknown ballot - might be from a proposal we haven't seen */
        return EKK_ERR_NOT_FOUND;
    }

    ekk_ballot_t *ballot = &cons->ballots[idx];

    /* Only proposer can receive votes */
    if (ballot->proposer != cons->my_id) {
        return EKK_ERR_INVALID_ARG;
    }

    /* Cannot vote on completed ballot */
    if (ballot->completed) {
        return EKK_OK;  /* Ignore late votes */
    }

    /* Find voter slot (use voter_id as index for simplicity) */
    uint32_t voter_slot = voter_id % EKK_K_NEIGHBORS;

    /* Check for duplicate vote */
    if (ballot->votes[voter_slot] != EKK_VOTE_ABSTAIN) {
        return EKK_OK;  /* Already voted */
    }

    /* Record vote */
    ballot->votes[voter_slot] = vote;
    ballot->vote_count++;

    switch (vote) {
        case EKK_VOTE_YES:
            ballot->yes_count++;
            break;
        case EKK_VOTE_NO:
            ballot->no_count++;
            break;
        default:
            break;
    }

    /* Check if we can determine result early */
    ekk_vote_result_t result = evaluate_ballot(ballot, EKK_K_NEIGHBORS);
    if (result != EKK_VOTE_PENDING) {
        finalize_ballot(cons, ballot, result);
    }

    return EKK_OK;
}

ekk_error_t ekk_consensus_on_proposal(ekk_consensus_t *cons,
                                       ekk_module_id_t proposer_id,
                                       ekk_ballot_id_t ballot_id,
                                       ekk_proposal_type_t type,
                                       uint32_t data,
                                       ekk_fixed_t threshold)
{
    if (cons == NULL || proposer_id == EKK_INVALID_MODULE_ID ||
        ballot_id == EKK_INVALID_BALLOT_ID) {
        return EKK_ERR_INVALID_ARG;
    }

    /* Ignore self proposals */
    if (proposer_id == cons->my_id) {
        return EKK_OK;
    }

    ekk_time_us_t now = ekk_hal_time_us();

    /* Check if inhibited */
    if (is_inhibited(cons, ballot_id, now)) {
        /* Send inhibit response */
        send_vote(cons, proposer_id, ballot_id, EKK_VOTE_INHIBIT);
        return EKK_ERR_INHIBITED;
    }

    /* Check if we already have this ballot */
    int idx = find_ballot_index(cons, ballot_id);
    if (idx >= 0) {
        /* Duplicate proposal */
        return EKK_OK;
    }

    /* Allocate slot for remote ballot tracking */
    idx = allocate_ballot_slot(cons);
    if (idx < 0) {
        /* No room - vote no */
        send_vote(cons, proposer_id, ballot_id, EKK_VOTE_NO);
        return EKK_ERR_BUSY;
    }

    /* Store ballot info */
    ekk_ballot_t *ballot = &cons->ballots[idx];
    memset(ballot, 0, sizeof(ekk_ballot_t));

    ballot->id = ballot_id;
    ballot->type = type;
    ballot->proposer = proposer_id;
    ballot->proposal_data = data;
    ballot->threshold = threshold;
    ballot->deadline = now + cons->config.vote_timeout;
    ballot->result = EKK_VOTE_PENDING;
    ballot->completed = false;

    cons->active_ballot_count++;

    /* Decide how to vote */
    ekk_vote_value_t my_vote = EKK_VOTE_ABSTAIN;

    if (g_decide_callback != NULL) {
        my_vote = g_decide_callback(cons, ballot);
    } else {
        /* Default: vote yes */
        my_vote = EKK_VOTE_YES;
    }

    /* Send vote */
    send_vote(cons, proposer_id, ballot_id, my_vote);

    return EKK_OK;
}

/* ============================================================================
 * QUERY
 * ============================================================================ */

ekk_vote_result_t ekk_consensus_get_result(const ekk_consensus_t *cons,
                                            ekk_ballot_id_t ballot_id)
{
    if (cons == NULL || ballot_id == EKK_INVALID_BALLOT_ID) {
        return EKK_VOTE_PENDING;
    }

    int idx = find_ballot_index(cons, ballot_id);
    if (idx < 0) {
        return EKK_VOTE_PENDING;  /* Unknown */
    }

    return cons->ballots[idx].result;
}

/* ============================================================================
 * PERIODIC TICK
 * ============================================================================ */

uint32_t ekk_consensus_tick(ekk_consensus_t *cons, ekk_time_us_t now)
{
    if (cons == NULL) {
        return 0;
    }

    uint32_t completed_count = 0;

    /* Check each active ballot for timeout */
    for (uint32_t i = 0; i < cons->active_ballot_count; i++) {
        ekk_ballot_t *ballot = &cons->ballots[i];

        if (ballot->completed) {
            continue;
        }

        /* Check for inhibition */
        if (is_inhibited(cons, ballot->id, now)) {
            finalize_ballot(cons, ballot, EKK_VOTE_CANCELLED);
            completed_count++;
            continue;
        }

        /* Check for timeout */
        if (now >= ballot->deadline) {
            /* Evaluate with votes received */
            ekk_vote_result_t result = evaluate_ballot(ballot, ballot->vote_count);

            if (result == EKK_VOTE_PENDING) {
                /* Not enough votes before timeout */
                result = EKK_VOTE_TIMEOUT;
            }

            finalize_ballot(cons, ballot, result);
            completed_count++;
        }
    }

    /* Clean up expired inhibitions */
    uint32_t write_idx = 0;
    for (uint32_t read_idx = 0; read_idx < cons->inhibit_count; read_idx++) {
        if (cons->inhibit_until[read_idx] > now) {
            if (write_idx != read_idx) {
                cons->inhibited[write_idx] = cons->inhibited[read_idx];
                cons->inhibit_until[write_idx] = cons->inhibit_until[read_idx];
            }
            write_idx++;
        }
    }
    cons->inhibit_count = write_idx;

    /* Clean up completed ballots periodically */
    if (completed_count > 0) {
        cleanup_completed_ballots(cons);
    }

    return completed_count;
}

/* ============================================================================
 * CALLBACKS
 * ============================================================================ */

void ekk_consensus_set_decide_callback(ekk_consensus_t *cons,
                                        ekk_consensus_decide_cb callback)
{
    EKK_UNUSED(cons);
    g_decide_callback = callback;
}

void ekk_consensus_set_complete_callback(ekk_consensus_t *cons,
                                          ekk_consensus_complete_cb callback)
{
    EKK_UNUSED(cons);
    g_complete_callback = callback;
}
