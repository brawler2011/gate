#include <iostream>
#include <vector>
using namespace std;

int main() {
    int a, b;
    if (cin >> a >> b) {
        // Allocate and touch ~300 MB of memory (exceeds 256MB limit).
        // Volatile writes keep the compiler from eliding the allocation.
        vector<char> v(300 * 1024 * 1024);
        volatile char *p = v.data();
        for (size_t i = 0; i < v.size(); i += 4096) {
            p[i] = 1;
        }
        cout << a + b << endl;
    }
    return 0;
}
