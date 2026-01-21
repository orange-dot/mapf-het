/**
 * @file bench_spsc.c
 * @brief EK-KOR v2 - SPSC Ring Buffer Benchmark
 *
 * @copyright Copyright (c) 2026 Elektrokombinacija
 * @license MIT
 *
 * Measures push/pop latency over 100K iterations.
 * Target: <100ns per operation on modern x86.
 */

#include "ekk/ekk_spsc.h"
#include "ekk/ekk_hal.h"

#include <stdio.h>
#include <stdlib.h>
#include <stdint.h>
#include <string.h>

/* ============================================================================
 * Test Configuration
 * ============================================================================ */

#define ITERATIONS      100000
#define QUEUE_CAPACITY  256     /* Must be power of 2 */
#define WARMUP_ITERS    1000

/* Test item structure (typical CAN frame size) */
typedef struct {
    uint32_t id;
    uint8_t  data[8];
    uint8_t  len;
    uint8_t  flags;
} test_item_t;

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

static void bench_push_pop_copy(void) {
    printf("\n=== SPSC Copy Benchmark (push/pop) ===\n");

    /* Allocate queue */
    test_item_t buffer[QUEUE_CAPACITY];
    ekk_spsc_t queue;

    if (ekk_spsc_init(&queue, buffer, QUEUE_CAPACITY, sizeof(test_item_t)) != EKK_OK) {
        printf("FAIL: Could not initialize queue\n");
        return;
    }

    /* Warmup */
    test_item_t item = {.id = 0, .len = 8, .flags = 0};
    memset(item.data, 0xAA, sizeof(item.data));

    for (int i = 0; i < WARMUP_ITERS; i++) {
        item.id = i;
        ekk_spsc_push(&queue, &item);
        ekk_spsc_pop(&queue, &item);
    }
    ekk_spsc_reset(&queue);

    /* Benchmark push */
    stats_t push_stats, pop_stats;
    stats_init(&push_stats);
    stats_init(&pop_stats);

    for (int i = 0; i < ITERATIONS; i++) {
        item.id = i;

        uint64_t t0 = get_time_ns();
        ekk_spsc_push(&queue, &item);
        uint64_t t1 = get_time_ns();

        stats_add(&push_stats, t1 - t0);
    }

    /* Benchmark pop */
    for (int i = 0; i < ITERATIONS; i++) {
        uint64_t t0 = get_time_ns();
        ekk_spsc_pop(&queue, &item);
        uint64_t t1 = get_time_ns();

        stats_add(&pop_stats, t1 - t0);
    }

    /* Results */
    printf("Iterations: %d\n", ITERATIONS);
    printf("Push: avg=%lluns, min=%lluns, max=%lluns\n",
           (unsigned long long)stats_avg(&push_stats),
           (unsigned long long)push_stats.min,
           (unsigned long long)push_stats.max);
    printf("Pop:  avg=%lluns, min=%lluns, max=%lluns\n",
           (unsigned long long)stats_avg(&pop_stats),
           (unsigned long long)pop_stats.min,
           (unsigned long long)pop_stats.max);

    uint64_t total_avg = stats_avg(&push_stats) + stats_avg(&pop_stats);
    if (total_avg < 200) {
        printf("RESULT: PASS (<100ns target met, total round-trip %lluns)\n",
               (unsigned long long)total_avg);
    } else {
        printf("RESULT: WARNING (total round-trip %lluns, target <200ns)\n",
               (unsigned long long)total_avg);
    }
}

static void bench_push_pop_zerocopy(void) {
    printf("\n=== SPSC Zero-Copy Benchmark (acquire/commit/peek/release) ===\n");

    /* Allocate queue */
    test_item_t buffer[QUEUE_CAPACITY];
    ekk_spsc_t queue;

    if (ekk_spsc_init(&queue, buffer, QUEUE_CAPACITY, sizeof(test_item_t)) != EKK_OK) {
        printf("FAIL: Could not initialize queue\n");
        return;
    }

    /* Warmup */
    for (int i = 0; i < WARMUP_ITERS; i++) {
        test_item_t *slot = ekk_spsc_push_acquire(&queue);
        if (slot) {
            slot->id = i;
            ekk_spsc_push_commit(&queue);
        }
        slot = ekk_spsc_pop_peek(&queue);
        if (slot) {
            ekk_spsc_pop_release(&queue);
        }
    }
    ekk_spsc_reset(&queue);

    /* Benchmark push (acquire+commit) */
    stats_t push_stats, pop_stats;
    stats_init(&push_stats);
    stats_init(&pop_stats);

    for (int i = 0; i < ITERATIONS; i++) {
        uint64_t t0 = get_time_ns();
        test_item_t *slot = ekk_spsc_push_acquire(&queue);
        if (slot) {
            slot->id = i;
            slot->len = 8;
            ekk_spsc_push_commit(&queue);
        }
        uint64_t t1 = get_time_ns();

        stats_add(&push_stats, t1 - t0);
    }

    /* Benchmark pop (peek+release) */
    for (int i = 0; i < ITERATIONS; i++) {
        uint64_t t0 = get_time_ns();
        test_item_t *slot = ekk_spsc_pop_peek(&queue);
        if (slot) {
            volatile uint32_t id = slot->id;  /* Force read */
            (void)id;
            ekk_spsc_pop_release(&queue);
        }
        uint64_t t1 = get_time_ns();

        stats_add(&pop_stats, t1 - t0);
    }

    /* Results */
    printf("Iterations: %d\n", ITERATIONS);
    printf("Push (acquire+commit): avg=%lluns, min=%lluns, max=%lluns\n",
           (unsigned long long)stats_avg(&push_stats),
           (unsigned long long)push_stats.min,
           (unsigned long long)push_stats.max);
    printf("Pop (peek+release):    avg=%lluns, min=%lluns, max=%lluns\n",
           (unsigned long long)stats_avg(&pop_stats),
           (unsigned long long)pop_stats.min,
           (unsigned long long)pop_stats.max);

    uint64_t total_avg = stats_avg(&push_stats) + stats_avg(&pop_stats);
    if (total_avg < 150) {
        printf("RESULT: PASS (<75ns per operation target met)\n");
    } else {
        printf("RESULT: WARNING (total round-trip %lluns)\n",
               (unsigned long long)total_avg);
    }
}

static void bench_throughput(void) {
    printf("\n=== SPSC Throughput Benchmark ===\n");

    test_item_t buffer[QUEUE_CAPACITY];
    ekk_spsc_t queue;

    if (ekk_spsc_init(&queue, buffer, QUEUE_CAPACITY, sizeof(test_item_t)) != EKK_OK) {
        printf("FAIL: Could not initialize queue\n");
        return;
    }

    test_item_t item = {.id = 0, .len = 8, .flags = 0};
    memset(item.data, 0xAA, sizeof(item.data));

    /* Measure time to push/pop 1M items in batches */
    const int total_items = 1000000;
    const int batch_size = QUEUE_CAPACITY / 2;

    uint64_t t_start = get_time_ns();

    int items_processed = 0;
    while (items_processed < total_items) {
        /* Push batch */
        for (int i = 0; i < batch_size && items_processed + i < total_items; i++) {
            item.id = items_processed + i;
            ekk_spsc_push(&queue, &item);
        }

        /* Pop batch */
        int popped = 0;
        while (ekk_spsc_pop(&queue, &item) == EKK_OK) {
            popped++;
        }

        items_processed += popped;
    }

    uint64_t t_end = get_time_ns();
    double elapsed_sec = (t_end - t_start) / 1000000000.0;
    double throughput = total_items / elapsed_sec / 1000000.0;

    printf("Total items: %d\n", total_items);
    printf("Elapsed: %.3f sec\n", elapsed_sec);
    printf("Throughput: %.2f M items/sec\n", throughput);

    if (throughput > 10.0) {
        printf("RESULT: PASS (>10M items/sec target met)\n");
    } else {
        printf("RESULT: WARNING (throughput %.2fM items/sec)\n", throughput);
    }
}

/* ============================================================================
 * Main
 * ============================================================================ */

int main(void) {
    printf("EK-KOR v2 SPSC Ring Buffer Benchmark\n");
    printf("=====================================\n");
    printf("Item size: %zu bytes\n", sizeof(test_item_t));
    printf("Queue capacity: %d items\n", QUEUE_CAPACITY);

    bench_push_pop_copy();
    bench_push_pop_zerocopy();
    bench_throughput();

    printf("\n=== Benchmark Complete ===\n");
    return 0;
}
