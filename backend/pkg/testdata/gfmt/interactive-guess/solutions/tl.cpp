#include <iostream>

int main() {
    int n;
    std::cin >> n;
    volatile unsigned long long spin = 0;
    while (true) spin = spin + 1;
}
