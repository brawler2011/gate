// Unpredictable indirect branches: the branch predictor and pipeline refills.
#define _GNU_SOURCE
#include <sched.h>
#include <stdint.h>
#include <stdio.h>

int main(void) {
	uint64_t x = 2463534242ull, s = 0;
	for (long i = 0; i < 17000000; i++) {
		x ^= x << 13;
		x ^= x >> 7;
		x ^= x << 17;
		switch (x >> 61) {
		case 0: s += x; break;
		case 1: s ^= x >> 3; break;
		case 2: s -= x << 1; break;
		case 3: s = s * 3 + 1; break;
		case 4: s ^= s >> 11; break;
		case 5: s += s << 7; break;
		case 6: s = ~s; break;
		default: s -= 12345; break;
		}
	}
	printf("%llu\n", (unsigned long long)s);
	fprintf(stderr, "%d\n", sched_getcpu());
	return 0;
}
