#include <iostream>
#include <vector>
#include <algorithm>

using namespace std;

int main() {
    int n;
    if (cin >> n) {
        vector<long long> a(n);
        for (int i = 0; i < n; i++) {
            cin >> a[i];
        }
        long long max_so_far = a[0];
        long long curr_max = a[0];
        for (int i = 1; i < n; i++) {
            curr_max = max(a[i], curr_max + a[i]);
            max_so_far = max(max_so_far, curr_max);
        }
        cout << max_so_far << endl;
    }
    return 0;
}
