// Pointer chasing through 32 MiB: memory latency past the caches.
#define _GNU_SOURCE
#include <sched.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

#define N (8u << 20)

int main(void) {
	uint32_t *next = malloc(N * sizeof *next);
	if (!next) return 1;
	for (uint32_t i = 0; i < N; i++) next[i] = i;
	// Sattolo's shuffle makes a single cycle, so the chase visits every element.
	uint64_t x = 88172645463325252ull;
	for (uint32_t i = N - 1; i > 0; i--) {
		x ^= x << 13;
		x ^= x >> 7;
		x ^= x << 17;
		uint32_t j = x % i, t = next[i];
		next[i] = next[j];
		next[j] = t;
	}
	uint32_t p = 0;
	for (long i = 0; i < 3000000; i++) p = next[p];
	printf("%u\n", p);
	fprintf(stderr, "%d\n", sched_getcpu());
	return 0;
}
