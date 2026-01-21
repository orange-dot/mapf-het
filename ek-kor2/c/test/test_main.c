/**
 * @file test_main.c
 * @brief EK-KOR v2 - Basic Test Suite
 *
 * @copyright Copyright (c) 2026 Elektrokombinacija
 *
 * Simple test executable to verify the build and basic functionality.
 */

#include <ekk/ekk.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

/* ============================================================================
 * TEST MACROS
 * ============================================================================ */

#define TEST_ASSERT(cond, msg) \
    do { \
        if (!(cond)) { \
            printf("FAIL: %s (%s:%d)\n", msg, __FILE__, __LINE__); \
            return 1; \
        } \
    } while (0)

#define TEST_PASS(name) printf("PASS: %s\n", name)

/* ============================================================================
 * TEST: System Initialization
 * ============================================================================ */

static int test_init(void)
{
    ekk_error_t err = ekk_init();
    TEST_ASSERT(err == EKK_OK, "ekk_init() should succeed");

    ekk_field_region_t *region = ekk_get_field_region();
    TEST_ASSERT(region != NULL, "Field region should be available");

    TEST_PASS("test_init");
    return 0;
}

/* ============================================================================
 * TEST: Module Creation
 * ============================================================================ */

static int test_module_create(void)
{
    ekk_module_t mod;
    ekk_position_t pos = {.x = 1, .y = 2, .z = 0};

    ekk_error_t err = ekk_module_init(&mod, 42, "test-module", pos);
    TEST_ASSERT(err == EKK_OK, "Module init should succeed");
    TEST_ASSERT(mod.id == 42, "Module ID should be 42");
    TEST_ASSERT(mod.state == EKK_MODULE_INIT, "Initial state should be INIT");

    TEST_PASS("test_module_create");
    return 0;
}

/* ============================================================================
 * TEST: Module Lifecycle
 * ============================================================================ */

static int test_module_lifecycle(void)
{
    ekk_module_t mod;
    ekk_position_t pos = {0, 0, 0};

    ekk_module_init(&mod, 1, "lifecycle-test", pos);

    /* Should be in INIT state */
    TEST_ASSERT(mod.state == EKK_MODULE_INIT, "Should start in INIT");

    /* Start module */
    ekk_error_t err = ekk_module_start(&mod);
    TEST_ASSERT(err == EKK_OK, "Start should succeed");
    TEST_ASSERT(mod.state == EKK_MODULE_DISCOVERING, "Should be DISCOVERING after start");

    /* Tick a few times */
    ekk_time_us_t now = ekk_hal_time_us();
    for (int i = 0; i < 10; i++) {
        err = ekk_module_tick(&mod, now);
        TEST_ASSERT(err == EKK_OK, "Tick should succeed");
        now += 1000;  /* 1ms */
    }

    /* Stop module */
    err = ekk_module_stop(&mod);
    TEST_ASSERT(err == EKK_OK, "Stop should succeed");
    TEST_ASSERT(mod.state == EKK_MODULE_SHUTDOWN, "Should be SHUTDOWN after stop");

    TEST_PASS("test_module_lifecycle");
    return 0;
}

/* ============================================================================
 * TEST: Fixed-Point Math
 * ============================================================================ */

extern ekk_fixed_t ekk_fixed_mul(ekk_fixed_t a, ekk_fixed_t b);
extern ekk_fixed_t ekk_fixed_div(ekk_fixed_t a, ekk_fixed_t b);

static int test_fixed_point(void)
{
    /* Test multiplication: 0.5 * 0.5 = 0.25 */
    ekk_fixed_t half = EKK_FIXED_HALF;
    ekk_fixed_t result = ekk_fixed_mul(half, half);
    ekk_fixed_t quarter = EKK_FIXED_ONE >> 2;

    /* Allow small error due to fixed-point precision */
    int32_t error = result - quarter;
    if (error < 0) error = -error;
    TEST_ASSERT(error < 10, "0.5 * 0.5 should be ~0.25");

    /* Test division: 1.0 / 2.0 = 0.5 */
    result = ekk_fixed_div(EKK_FIXED_ONE, EKK_FIXED_ONE * 2);
    error = result - half;
    if (error < 0) error = -error;
    TEST_ASSERT(error < 10, "1.0 / 2.0 should be ~0.5");

    TEST_PASS("test_fixed_point");
    return 0;
}

/* ============================================================================
 * TEST: Field Operations
 * ============================================================================ */

static int test_field_operations(void)
{
    ekk_field_t field;
    memset(&field, 0, sizeof(field));

    /* Set some values */
    field.components[EKK_FIELD_LOAD] = EKK_FIXED_HALF;
    field.components[EKK_FIELD_THERMAL] = EKK_FIXED_ONE >> 2;  /* 0.25 */
    field.source = 1;
    field.timestamp = ekk_hal_time_us();

    /* Publish */
    ekk_error_t err = ekk_field_publish(1, &field);
    TEST_ASSERT(err == EKK_OK, "Field publish should succeed");

    /* Sample */
    ekk_field_t sampled;
    err = ekk_field_sample(1, &sampled);
    TEST_ASSERT(err == EKK_OK, "Field sample should succeed");
    TEST_ASSERT(sampled.source == 1, "Sampled source should match");

    TEST_PASS("test_field_operations");
    return 0;
}

/* ============================================================================
 * TEST: Topology
 * ============================================================================ */

static int test_topology(void)
{
    ekk_topology_t topo;
    ekk_position_t pos = {0, 0, 0};

    ekk_error_t err = ekk_topology_init(&topo, 1, pos, NULL);
    TEST_ASSERT(err == EKK_OK, "Topology init should succeed");
    TEST_ASSERT(topo.my_id == 1, "Topology module ID should be 1");
    TEST_ASSERT(topo.neighbor_count == 0, "Initial neighbor count should be 0");

    /* Discover some modules */
    ekk_position_t pos2 = {1, 0, 0};
    ekk_position_t pos3 = {2, 0, 0};

    err = ekk_topology_on_discovery(&topo, 2, pos2);
    TEST_ASSERT(err == EKK_OK, "Discovery of module 2 should succeed");

    err = ekk_topology_on_discovery(&topo, 3, pos3);
    TEST_ASSERT(err == EKK_OK, "Discovery of module 3 should succeed");

    /* Reelect should add them as neighbors */
    uint32_t count = ekk_topology_reelect(&topo);
    TEST_ASSERT(count >= 2, "Should have at least 2 neighbors");

    TEST_PASS("test_topology");
    return 0;
}

/* ============================================================================
 * TEST: Consensus
 * ============================================================================ */

static int test_consensus(void)
{
    ekk_consensus_t cons;

    ekk_error_t err = ekk_consensus_init(&cons, 1, NULL);
    TEST_ASSERT(err == EKK_OK, "Consensus init should succeed");

    /* Propose something */
    ekk_ballot_id_t ballot_id;
    err = ekk_consensus_propose(&cons, EKK_PROPOSAL_MODE_CHANGE, 42,
                                 EKK_THRESHOLD_SIMPLE_MAJORITY, &ballot_id);
    TEST_ASSERT(err == EKK_OK, "Proposal should succeed");
    TEST_ASSERT(ballot_id != EKK_INVALID_BALLOT_ID, "Ballot ID should be valid");

    /* Check result (should be pending) */
    ekk_vote_result_t result = ekk_consensus_get_result(&cons, ballot_id);
    TEST_ASSERT(result == EKK_VOTE_PENDING || result == EKK_VOTE_APPROVED,
                "Result should be pending or approved (self-vote)");

    TEST_PASS("test_consensus");
    return 0;
}

/* ============================================================================
 * TEST: Heartbeat
 * ============================================================================ */

static int test_heartbeat(void)
{
    ekk_heartbeat_t hb;

    ekk_error_t err = ekk_heartbeat_init(&hb, 1, NULL);
    TEST_ASSERT(err == EKK_OK, "Heartbeat init should succeed");

    /* Add neighbor */
    err = ekk_heartbeat_add_neighbor(&hb, 2);
    TEST_ASSERT(err == EKK_OK, "Add neighbor should succeed");

    /* Check initial health */
    ekk_health_state_t health = ekk_heartbeat_get_health(&hb, 2);
    TEST_ASSERT(health == EKK_HEALTH_UNKNOWN, "Initial health should be UNKNOWN");

    /* Receive heartbeat */
    ekk_time_us_t now = ekk_hal_time_us();
    err = ekk_heartbeat_received(&hb, 2, 1, now);
    TEST_ASSERT(err == EKK_OK, "Heartbeat received should succeed");

    /* Check health after heartbeat */
    health = ekk_heartbeat_get_health(&hb, 2);
    TEST_ASSERT(health == EKK_HEALTH_ALIVE, "Health should be ALIVE after heartbeat");

    TEST_PASS("test_heartbeat");
    return 0;
}

/* ============================================================================
 * TEST: Task Management
 * ============================================================================ */

static volatile int g_task_run_count = 0;

static void test_task_fn(void *arg)
{
    (void)arg;
    g_task_run_count++;
}

static int test_task_management(void)
{
    ekk_module_t mod;
    ekk_position_t pos = {0, 0, 0};

    ekk_module_init(&mod, 1, "task-test", pos);

    /* Add a task */
    ekk_task_id_t task_id;
    ekk_error_t err = ekk_module_add_task(&mod, "test-task", test_task_fn,
                                           NULL, 0, 0, &task_id);
    TEST_ASSERT(err == EKK_OK, "Add task should succeed");
    TEST_ASSERT(task_id == 0, "First task ID should be 0");

    /* Make task ready */
    err = ekk_module_task_ready(&mod, task_id);
    TEST_ASSERT(err == EKK_OK, "Task ready should succeed");

    /* Start module and tick */
    ekk_module_start(&mod);
    g_task_run_count = 0;

    ekk_time_us_t now = ekk_hal_time_us();
    ekk_module_tick(&mod, now);

    TEST_ASSERT(g_task_run_count == 1, "Task should have run once");

    TEST_PASS("test_task_management");
    return 0;
}

/* ============================================================================
 * MAIN
 * ============================================================================ */

int main(int argc, char **argv)
{
    (void)argc;
    (void)argv;

    printf("EK-KOR v2 Test Suite\n");
    printf("====================\n\n");

    int failures = 0;

    failures += test_init();
    failures += test_fixed_point();
    failures += test_field_operations();
    failures += test_topology();
    failures += test_consensus();
    failures += test_heartbeat();
    failures += test_module_create();
    failures += test_module_lifecycle();
    failures += test_task_management();

    printf("\n====================\n");
    if (failures == 0) {
        printf("All tests PASSED\n");
        return 0;
    } else {
        printf("%d test(s) FAILED\n", failures);
        return 1;
    }
}
