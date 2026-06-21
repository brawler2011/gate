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
        // Wrong solution: assumes maximum sum is at least 0 (empty subarray)
        long long max_so_far = 0;
        long long curr_max = 0;
        for (int i = 0; i < n; i++) {
            curr_max = max(0LL, curr_max + a[i]);
            max_so_far = max(max_so_far, curr_max);
        }
        cout << max_so_far << endl;
    }
    return 0;
}
