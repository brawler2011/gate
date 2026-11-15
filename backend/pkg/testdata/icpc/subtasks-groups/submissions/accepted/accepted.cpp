#include <iostream>

using namespace std;

int main() {
    int n;
    if (cin >> n) {
        if (n % 2 == 0) {
            cout << "Even" << endl;
        } else {
            cout << "Odd" << endl;
        }
    }
    return 0;
}
