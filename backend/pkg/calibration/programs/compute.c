// Dependent integer arithmetic: the core's ALUs and nothing else.
#define _GNU_SOURCE
#include <sched.h>
#include <stdint.h>
#include <stdio.h>

int main(void) {
	uint64_t x = 88172645463325252ull, s = 0;
	for (long i = 0; i < 70000000; i++) {
		x ^= x << 13;
		x ^= x >> 7;
		x ^= x << 17;
		s += x * 2685821657736338717ull;
	}
	printf("%llu\n", (unsigned long long)s);
	fprintf(stderr, "%d\n", sched_getcpu());
	return 0;
}
