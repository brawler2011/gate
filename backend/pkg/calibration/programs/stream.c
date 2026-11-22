// Sequential passes over 64 MiB: memory bandwidth.
#define _GNU_SOURCE
#include <sched.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>

#define N (8u << 20)

int main(void) {
	uint64_t *a = malloc(N * sizeof *a);
	if (!a) return 1;
	for (uint32_t i = 0; i < N; i++) a[i] = i;
	uint64_t s = 0;
	for (int pass = 0; pass < 50; pass++)
		for (uint32_t i = 0; i < N; i++) s += a[i] ^ pass;
	printf("%llu\n", (unsigned long long)s);
	fprintf(stderr, "%d\n", sched_getcpu());
	return 0;
}
