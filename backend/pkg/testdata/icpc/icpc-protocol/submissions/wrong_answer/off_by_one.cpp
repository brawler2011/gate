#include <cstdio>

int main() {
    int a, b;
    std::scanf("%d %d", &a, &b);
    std::printf("%d\n", a + b + (a > 5));
}
