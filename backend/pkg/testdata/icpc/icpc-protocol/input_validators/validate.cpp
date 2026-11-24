#include <cstdio>

int main() {
    int a, b;
    char end;
    bool ok = std::scanf("%d %d", &a, &b) == 2 && 0 <= a && a <= 1000 && 0 <= b && b <= 1000;
    ok = ok && std::scanf("%c", &end) == 1 && end == '\n' && std::scanf("%c", &end) == EOF;
    return ok ? 42 : 43;
}
