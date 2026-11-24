#include <cstdio>

int main() {
    long long lo = 1, hi;
    std::scanf("%lld", &hi);
    while (lo < hi) {
        long long mid = (lo + hi) / 2;
        std::printf("? %lld\n", mid);
        std::fflush(stdout);
        char reply;
        std::scanf(" %c", &reply);
        if (reply == '=') lo = hi = mid;
        else if (reply == '<') hi = mid - 1;
        else lo = mid + 1;
    }
    std::printf("! %lld\n", lo);
    std::fflush(stdout);
}
