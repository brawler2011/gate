#include <iostream>
using namespace std;

int main() {
    int a, b;
    cin >> a >> b;
    volatile int x = 0;
    while (true) {
        x++;
    }
    cout << a + b << endl;
}
