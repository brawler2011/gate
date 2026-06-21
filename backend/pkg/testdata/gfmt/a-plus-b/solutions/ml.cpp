#include <iostream>
#include <vector>
using namespace std;

int main() {
    int a, b;
    if (cin >> a >> b) {
        // Allocate ~300 MB of memory (exceeds 256MB limit)
        // This will throw std::bad_alloc (resulting in RE in rlimit mode, or MLE in cgroup mode)
        vector<char> v(300 * 1024 * 1024, 1);
        cout << a + b + v[0] - 1 << endl;
    }
    return 0;
}
