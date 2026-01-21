/**
 * @file bench_auth.c
 * @brief EK-KOR v2 - Chaskey MAC Benchmark
 *
 * @copyright Copyright (c) 2026 Elektrokombinacija
 * @license MIT
 *
 * Measures MAC computation for various message sizes.
 * Target: <2μs per 16-byte block on modern x86.
 */

#include "ekk/ekk_auth.h"

#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>

/* ============================================================================
 * Test Configuration
 * ============================================================================ */

#define ITERATIONS      100000
#define WARMUP_ITERS    1000

/* Message sizes to benchmark */
static const size_t MESSAGE_SIZES[] = {0, 8, 16, 32, 64, 128, 256};
#define NUM_SIZES (sizeof(MESSAGE_SIZES) / sizeof(MESSAGE_SIZES[0]))

/* ============================================================================
 * Timing Helpers
 * ============================================================================ */

#ifdef _WIN32
#include <windows.h>

static uint64_t get_time_ns(void) {
    static LARGE_INTEGER freq = {0};
    if (freq.QuadPart == 0) {
        QueryPerformanceFrequency(&freq);
    }
    LARGE_INTEGER counter;
    QueryPerformanceCounter(&counter);
    return (uint64_t)(counter.QuadPart * 1000000000ULL / freq.QuadPart);
}

#else
#include <time.h>

static uint64_t get_time_ns(void) {
    struct timespec ts;
    clock_gettime(CLOCK_MONOTONIC, &ts);
    return (uint64_t)ts.tv_sec * 1000000000ULL + ts.tv_nsec;
}
#endif

/* ============================================================================
 * Statistics
 * ============================================================================ */

typedef struct {
    uint64_t min;
    uint64_t max;
    uint64_t sum;
    uint64_t count;
} stats_t;

static void stats_init(stats_t *s) {
    s->min = UINT64_MAX;
    s->max = 0;
    s->sum = 0;
    s->count = 0;
}

static void stats_add(stats_t *s, uint64_t value) {
    if (value < s->min) s->min = value;
    if (value > s->max) s->max = value;
    s->sum += value;
    s->count++;
}

static uint64_t stats_avg(const stats_t *s) {
    return s->count > 0 ? s->sum / s->count : 0;
}

/* ============================================================================
 * Benchmark Functions
 * ============================================================================ */

static void bench_oneshot(void) {
    printf("\n=== Chaskey MAC One-Shot Benchmark ===\n");
    printf("%-10s %12s %12s %12s %12s\n",
           "Size", "Avg (ns)", "Min (ns)", "Max (ns)", "MB/s");
    printf("%-10s %12s %12s %12s %12s\n",
           "----", "--------", "--------", "--------", "----");

    /* Initialize key */
    uint8_t raw_key[16] = {
        0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
        0x09, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16
    };
    ekk_auth_key_t key;
    ekk_auth_key_init(&key, raw_key);

    /* Message buffer */
    uint8_t message[256];
    memset(message, 0xAA, sizeof(message));

    ekk_auth_tag_t tag;

    for (size_t i = 0; i < NUM_SIZES; i++) {
        size_t msg_size = MESSAGE_SIZES[i];
        stats_t stats;
        stats_init(&stats);

        /* Warmup */
        for (int j = 0; j < WARMUP_ITERS; j++) {
            ekk_auth_compute(&key, message, msg_size, &tag);
        }

        /* Benchmark */
        for (int j = 0; j < ITERATIONS; j++) {
            uint64_t t0 = get_time_ns();
            ekk_auth_compute(&key, message, msg_size, &tag);
            uint64_t t1 = get_time_ns();

            stats_add(&stats, t1 - t0);
        }

        /* Calculate throughput */
        double avg_us = stats_avg(&stats) / 1000.0;
        double throughput = 0.0;
        if (msg_size > 0 && avg_us > 0) {
            throughput = (msg_size / avg_us);  /* MB/s */
        }

        printf("%-10zu %12llu %12llu %12llu %12.2f\n",
               msg_size,
               (unsigned long long)stats_avg(&stats),
               (unsigned long long)stats.min,
               (unsigned long long)stats.max,
               throughput);
    }

    ekk_auth_key_clear(&key);
}

static void bench_incremental(void) {
    printf("\n=== Chaskey MAC Incremental Benchmark ===\n");
    printf("(Simulates chunked message processing)\n\n");

    /* Initialize key */
    uint8_t raw_key[16] = {
        0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
        0x09, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16
    };
    ekk_auth_key_t key;
    ekk_auth_key_init(&key, raw_key);

    /* Message: 64 bytes in 4x16-byte chunks */
    uint8_t chunks[4][16];
    for (int i = 0; i < 4; i++) {
        memset(chunks[i], 0x10 + i, 16);
    }

    ekk_auth_ctx_t ctx;
    ekk_auth_tag_t tag;
    stats_t stats;
    stats_init(&stats);

    /* Warmup */
    for (int j = 0; j < WARMUP_ITERS; j++) {
        ekk_auth_init(&ctx, &key);
        for (int i = 0; i < 4; i++) {
            ekk_auth_update(&ctx, chunks[i], 16);
        }
        ekk_auth_final(&ctx, &tag);
    }

    /* Benchmark */
    for (int j = 0; j < ITERATIONS; j++) {
        uint64_t t0 = get_time_ns();

        ekk_auth_init(&ctx, &key);
        for (int i = 0; i < 4; i++) {
            ekk_auth_update(&ctx, chunks[i], 16);
        }
        ekk_auth_final(&ctx, &tag);

        uint64_t t1 = get_time_ns();
        stats_add(&stats, t1 - t0);
    }

    printf("64-byte message in 4x16-byte chunks:\n");
    printf("  Avg: %llu ns\n", (unsigned long long)stats_avg(&stats));
    printf("  Min: %llu ns\n", (unsigned long long)stats.min);
    printf("  Max: %llu ns\n", (unsigned long long)stats.max);

    /* Compare with one-shot for same data */
    stats_t oneshot_stats;
    stats_init(&oneshot_stats);

    uint8_t full_msg[64];
    memcpy(full_msg, chunks, 64);

    for (int j = 0; j < ITERATIONS; j++) {
        uint64_t t0 = get_time_ns();
        ekk_auth_compute(&key, full_msg, 64, &tag);
        uint64_t t1 = get_time_ns();
        stats_add(&oneshot_stats, t1 - t0);
    }

    printf("\nOne-shot comparison (same 64 bytes):\n");
    printf("  Avg: %llu ns\n", (unsigned long long)stats_avg(&oneshot_stats));

    double overhead = 100.0 * (stats_avg(&stats) - stats_avg(&oneshot_stats)) /
                      stats_avg(&oneshot_stats);
    printf("\nIncremental overhead: %.1f%%\n", overhead);

    ekk_auth_key_clear(&key);
}

static void bench_verify(void) {
    printf("\n=== Chaskey MAC Verify Benchmark ===\n");

    /* Initialize key */
    uint8_t raw_key[16] = {
        0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
        0x09, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16
    };
    ekk_auth_key_t key;
    ekk_auth_key_init(&key, raw_key);

    /* Test message */
    uint8_t message[16] = "Hello, EK-KOR!";
    ekk_auth_tag_t tag;

    /* Compute valid tag */
    ekk_auth_compute(&key, message, 16, &tag);

    stats_t valid_stats, invalid_stats;
    stats_init(&valid_stats);
    stats_init(&invalid_stats);

    /* Benchmark valid verification */
    for (int j = 0; j < ITERATIONS; j++) {
        uint64_t t0 = get_time_ns();
        volatile bool result = ekk_auth_verify(&key, message, 16, &tag);
        uint64_t t1 = get_time_ns();
        (void)result;

        stats_add(&valid_stats, t1 - t0);
    }

    /* Benchmark invalid verification (wrong tag) */
    ekk_auth_tag_t bad_tag;
    memset(&bad_tag, 0xFF, sizeof(bad_tag));

    for (int j = 0; j < ITERATIONS; j++) {
        uint64_t t0 = get_time_ns();
        volatile bool result = ekk_auth_verify(&key, message, 16, &bad_tag);
        uint64_t t1 = get_time_ns();
        (void)result;

        stats_add(&invalid_stats, t1 - t0);
    }

    printf("Valid tag verification:\n");
    printf("  Avg: %llu ns\n", (unsigned long long)stats_avg(&valid_stats));
    printf("  Min: %llu ns\n", (unsigned long long)valid_stats.min);

    printf("\nInvalid tag verification:\n");
    printf("  Avg: %llu ns\n", (unsigned long long)stats_avg(&invalid_stats));
    printf("  Min: %llu ns\n", (unsigned long long)invalid_stats.min);

    /* Check for timing leak (should be constant-time) */
    int64_t diff = (int64_t)stats_avg(&valid_stats) - (int64_t)stats_avg(&invalid_stats);
    double diff_pct = 100.0 * diff / stats_avg(&valid_stats);

    printf("\nTiming difference: %.1f%% ", diff_pct);
    if (diff_pct > -5.0 && diff_pct < 5.0) {
        printf("(PASS: constant-time within 5%%)\n");
    } else {
        printf("(WARNING: potential timing leak)\n");
    }

    ekk_auth_key_clear(&key);
}

static void bench_message_auth(void) {
    printf("\n=== EK-KOR Message Authentication Benchmark ===\n");

    /* Initialize key */
    uint8_t raw_key[16] = {
        0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08,
        0x09, 0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16
    };
    ekk_auth_key_t key;
    ekk_auth_key_init(&key, raw_key);

    /* Typical vote message payload */
    uint8_t vote_data[8] = {0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08};
    uint8_t sender_id = 42;
    uint8_t msg_type = 5;  /* EKK_MSG_VOTE */

    ekk_auth_tag_t tag;
    stats_t stats;
    stats_init(&stats);

    /* Warmup */
    for (int j = 0; j < WARMUP_ITERS; j++) {
        ekk_auth_message(&key, sender_id, msg_type, vote_data, 8, &tag);
    }

    /* Benchmark */
    for (int j = 0; j < ITERATIONS; j++) {
        uint64_t t0 = get_time_ns();
        ekk_auth_message(&key, sender_id, msg_type, vote_data, 8, &tag);
        uint64_t t1 = get_time_ns();

        stats_add(&stats, t1 - t0);
    }

    printf("Vote message (8 bytes payload):\n");
    printf("  Avg: %llu ns\n", (unsigned long long)stats_avg(&stats));
    printf("  Min: %llu ns\n", (unsigned long long)stats.min);
    printf("  Max: %llu ns\n", (unsigned long long)stats.max);

    /* Target: <2μs = <2000ns */
    if (stats_avg(&stats) < 2000) {
        printf("\nRESULT: PASS (<2us target met)\n");
    } else {
        printf("\nRESULT: WARNING (avg %lluns, target <2000ns)\n",
               (unsigned long long)stats_avg(&stats));
    }

    ekk_auth_key_clear(&key);
}

/* ============================================================================
 * Main
 * ============================================================================ */

int main(void) {
    printf("EK-KOR v2 Chaskey MAC Benchmark\n");
    printf("================================\n");
    printf("Chaskey rounds: %d\n", EKK_CHASKEY_ROUNDS);
    printf("MAC tag size: %d bytes\n", EKK_MAC_TAG_SIZE);
    printf("Iterations: %d\n", ITERATIONS);

    bench_oneshot();
    bench_incremental();
    bench_verify();
    bench_message_auth();

    printf("\n=== Benchmark Complete ===\n");
    return 0;
}
