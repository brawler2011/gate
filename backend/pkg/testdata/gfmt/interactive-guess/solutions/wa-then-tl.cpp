#include <iostream>

// The interactor rejects the guess and exits before the time limit kills the loop.
int main() {
    int n;
    std::cin >> n;
    std::cout << "! 1" << std::endl;
    volatile unsigned long long spin = 0;
    while (true) spin = spin + 1;
}
