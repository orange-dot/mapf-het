/**
 * @file ekk_consensus.h
 * @brief EK-KOR v2 - Threshold-Based Distributed Consensus
 *
 * @copyright Copyright (c) 2026 Elektrokombinacija
 * @license MIT
 *
 * NOVELTY: Threshold Consensus for Mixed-Criticality Systems
 *
 * Modules vote on system-wide decisions using density-dependent threshold
 * mechanism. Supports supermajority for safety-critical decisions and
 * mutual inhibition for competing proposals.
 *
 * PATENT CLAIMS:
 * 3. "A threshold-based consensus mechanism for mixed-criticality embedded
 *    systems using density-dependent activation functions"
 *
 * Dependent claims:
 * - Mutual inhibition signals for competing operational modes
 * - Weighted voting based on module health state
 */

#ifndef EKK_CONSENSUS_H
#define EKK_CONSENSUS_H

#include "ekk_types.h"

#ifdef __cplusplus
extern "C" {
#endif

/* ============================================================================
 * CONSENSUS CONFIGURATION
 * ============================================================================ */

/**
 * @brief Standard threshold values
 */
#define EKK_THRESHOLD_SIMPLE_MAJORITY   EKK_FLOAT_TO_FIXED(0.50f)
#define EKK_THRESHOLD_SUPERMAJORITY     EKK_FLOAT_TO_FIXED(0.67f)
#define EKK_THRESHOLD_UNANIMOUS         EKK_FLOAT_TO_FIXED(1.00f)

/**
 * @brief Proposal types (application can extend)
 */
typedef enum {
    EKK_PROPOSAL_MODE_CHANGE    = 0,    /**< Change operational mode */
    EKK_PROPOSAL_POWER_LIMIT    = 1,    /**< Set cluster power limit */
    EKK_PROPOSAL_SHUTDOWN       = 2,    /**< Graceful shutdown */
    EKK_PROPOSAL_REFORMATION    = 3,    /**< Mesh reformation */
    EKK_PROPOSAL_CUSTOM_0       = 16,   /**< Application-defined */
    EKK_PROPOSAL_CUSTOM_1       = 17,
    EKK_PROPOSAL_CUSTOM_2       = 18,
    EKK_PROPOSAL_CUSTOM_3       = 19,
} ekk_proposal_type_t;

/**
 * @brief Consensus configuration
 */
typedef struct {
    ekk_time_us_t vote_timeout;         /**< Timeout for vote collection */
    ekk_time_us_t inhibit_duration;     /**< How long inhibition lasts */
    bool allow_self_vote;               /**< Can proposer vote for own proposal */
    bool require_all_neighbors;         /**< Require votes from all neighbors */
} ekk_consensus_config_t;

#define EKK_CONSENSUS_CONFIG_DEFAULT { \
    .vote_timeout = EKK_VOTE_TIMEOUT_US, \
    .inhibit_duration = 100000, /* 100ms */ \
    .allow_self_vote = true, \
    .require_all_neighbors = false, \
}

/* ============================================================================
 * BALLOT STRUCTURE
 * ============================================================================ */

/**
 * @brief Ballot (voting round)
 */
typedef struct {
    ekk_ballot_id_t id;                     /**< Unique ballot ID */
    ekk_proposal_type_t type;               /**< What we're voting on */
    ekk_module_id_t proposer;               /**< Who proposed it */

    uint32_t proposal_data;                 /**< Proposal-specific data */

    ekk_fixed_t threshold;                  /**< Required approval threshold */
    ekk_time_us_t deadline;                 /**< When voting ends */

    /* Vote tracking */
    uint8_t votes[EKK_K_NEIGHBORS];         /**< Votes from neighbors */
    uint8_t vote_count;                     /**< Votes received */
    uint8_t yes_count;                      /**< Approvals */
    uint8_t no_count;                       /**< Rejections */

    ekk_vote_result_t result;               /**< Final result */
    bool completed;                         /**< Voting finished */
} ekk_ballot_t;

/* ============================================================================
 * CONSENSUS STATE
 * ============================================================================ */

/**
 * @brief Consensus engine state
 */
typedef struct {
    ekk_module_id_t my_id;                      /**< This module's ID */

    ekk_ballot_t ballots[EKK_MAX_BALLOTS];      /**< Active ballots */
    uint32_t active_ballot_count;

    ekk_ballot_id_t inhibited[EKK_MAX_BALLOTS]; /**< Inhibited ballot IDs */
    ekk_time_us_t inhibit_until[EKK_MAX_BALLOTS];
    uint32_t inhibit_count;

    ekk_ballot_id_t next_ballot_id;             /**< Next ballot ID to use */

    ekk_consensus_config_t config;              /**< Configuration */
} ekk_consensus_t;

/* ============================================================================
 * CONSENSUS API
 * ============================================================================ */

/**
 * @brief Initialize consensus engine
 *
 * @param cons Consensus state (caller-allocated)
 * @param my_id This module's ID
 * @param config Configuration (or NULL for defaults)
 * @return EKK_OK on success
 */
ekk_error_t ekk_consensus_init(ekk_consensus_t *cons,
                                ekk_module_id_t my_id,
                                const ekk_consensus_config_t *config);

/**
 * @brief Propose a vote to k-neighbors
 *
 * Broadcasts proposal to all neighbors and waits for votes.
 * Returns immediately; check result via ekk_consensus_get_result().
 *
 * @param cons Consensus state
 * @param type Proposal type
 * @param data Proposal-specific data
 * @param threshold Required approval ratio (Q16.16, e.g., 0.67 for 2/3)
 * @param[out] ballot_id Assigned ballot ID
 * @return EKK_OK on success, EKK_ERR_BUSY if too many active ballots
 *
 * EXAMPLE:
 * @code
 * ekk_ballot_id_t ballot;
 * ekk_consensus_propose(&cons, EKK_PROPOSAL_MODE_CHANGE, MODE_REDUCED_POWER,
 *                       EKK_THRESHOLD_SUPERMAJORITY, &ballot);
 *
 * // Later...
 * ekk_vote_result_t result = ekk_consensus_get_result(&cons, ballot);
 * if (result == EKK_VOTE_APPROVED) {
 *     // Consensus reached!
 * }
 * @endcode
 */
ekk_error_t ekk_consensus_propose(ekk_consensus_t *cons,
                                   ekk_proposal_type_t type,
                                   uint32_t data,
                                   ekk_fixed_t threshold,
                                   ekk_ballot_id_t *ballot_id);

/**
 * @brief Cast vote in response to neighbor's proposal
 *
 * @param cons Consensus state
 * @param ballot_id Ballot to vote on
 * @param vote Vote value
 * @return EKK_OK on success, EKK_ERR_NOT_FOUND if ballot unknown
 */
ekk_error_t ekk_consensus_vote(ekk_consensus_t *cons,
                                ekk_ballot_id_t ballot_id,
                                ekk_vote_value_t vote);

/**
 * @brief Inhibit a competing proposal
 *
 * Blocks a proposal from reaching quorum. Used for mutual exclusion
 * between competing proposals (e.g., can't enter two modes at once).
 *
 * @param cons Consensus state
 * @param ballot_id Ballot to inhibit
 * @return EKK_OK on success
 */
ekk_error_t ekk_consensus_inhibit(ekk_consensus_t *cons,
                                   ekk_ballot_id_t ballot_id);

/**
 * @brief Process incoming vote message
 *
 * Called when receiving a vote from a neighbor.
 *
 * @param cons Consensus state
 * @param voter_id Voter's module ID
 * @param ballot_id Ballot being voted on
 * @param vote Vote value
 * @return EKK_OK on success
 */
ekk_error_t ekk_consensus_on_vote(ekk_consensus_t *cons,
                                   ekk_module_id_t voter_id,
                                   ekk_ballot_id_t ballot_id,
                                   ekk_vote_value_t vote);

/**
 * @brief Process incoming proposal message
 *
 * Called when receiving a proposal from a neighbor.
 *
 * @param cons Consensus state
 * @param proposer_id Proposer's module ID
 * @param ballot_id Ballot ID
 * @param type Proposal type
 * @param data Proposal data
 * @param threshold Required threshold
 * @return EKK_OK on success
 */
ekk_error_t ekk_consensus_on_proposal(ekk_consensus_t *cons,
                                       ekk_module_id_t proposer_id,
                                       ekk_ballot_id_t ballot_id,
                                       ekk_proposal_type_t type,
                                       uint32_t data,
                                       ekk_fixed_t threshold);

/**
 * @brief Get result of a ballot
 *
 * @param cons Consensus state
 * @param ballot_id Ballot to check
 * @return Vote result (PENDING if still voting)
 */
ekk_vote_result_t ekk_consensus_get_result(const ekk_consensus_t *cons,
                                            ekk_ballot_id_t ballot_id);

/**
 * @brief Periodic tick (call from main loop)
 *
 * Checks for timeouts and finalizes ballots.
 *
 * @param cons Consensus state
 * @param now Current timestamp
 * @return Number of ballots that completed this tick
 */
uint32_t ekk_consensus_tick(ekk_consensus_t *cons, ekk_time_us_t now);

/* ============================================================================
 * VOTE MESSAGE FORMAT
 * ============================================================================ */

/**
 * @brief Vote message (sent to proposer)
 */
EKK_PACK_BEGIN
typedef struct {
    uint8_t msg_type;               /**< EKK_MSG_VOTE */
    ekk_module_id_t voter_id;       /**< Voter's ID */
    ekk_ballot_id_t ballot_id;      /**< Which ballot */
    uint8_t vote;                   /**< The vote (ekk_vote_value_t) */
    uint32_t timestamp;             /**< Voting timestamp (truncated) */
} EKK_PACKED ekk_vote_msg_t;
EKK_PACK_END

EKK_STATIC_ASSERT(sizeof(ekk_vote_msg_t) <= 12, "Vote message too large");

/**
 * @brief Proposal message (broadcast to neighbors)
 */
EKK_PACK_BEGIN
typedef struct {
    uint8_t msg_type;               /**< EKK_MSG_PROPOSAL */
    ekk_module_id_t proposer_id;    /**< Proposer's ID */
    ekk_ballot_id_t ballot_id;      /**< Ballot ID */
    uint8_t type;                   /**< Proposal type (ekk_proposal_type_t) */
    uint32_t data;                  /**< Proposal data */
    ekk_fixed_t threshold;          /**< Required threshold */
} EKK_PACKED ekk_proposal_msg_t;
EKK_PACK_END

EKK_STATIC_ASSERT(sizeof(ekk_proposal_msg_t) <= 16, "Proposal message too large");

/* ============================================================================
 * CALLBACKS
 * ============================================================================ */

/**
 * @brief Callback when proposal needs local decision
 *
 * Application implements this to decide how to vote on proposals.
 */
typedef ekk_vote_value_t (*ekk_consensus_decide_cb)(ekk_consensus_t *cons,
                                                     const ekk_ballot_t *ballot);

/**
 * @brief Callback when ballot completes
 */
typedef void (*ekk_consensus_complete_cb)(ekk_consensus_t *cons,
                                           const ekk_ballot_t *ballot,
                                           ekk_vote_result_t result);

/**
 * @brief Set decision callback
 */
void ekk_consensus_set_decide_callback(ekk_consensus_t *cons,
                                        ekk_consensus_decide_cb callback);

/**
 * @brief Set completion callback
 */
void ekk_consensus_set_complete_callback(ekk_consensus_t *cons,
                                          ekk_consensus_complete_cb callback);

#ifdef __cplusplus
}
#endif

#endif /* EKK_CONSENSUS_H */
