#include <bits/stdc++.h>
using namespace std;

const int mxn = 9 + 1e6;

void SaKaTa() {
  int a, b;
  cin >> a >> b;
  cout << a + b;
}

int32_t main() {
#define task "SaKaTa"
  cin.tie(0)->sync_with_stdio(0);
  if (fopen(task".inp", "r")) {
    freopen(task".inp", "r", stdin);
    freopen(task".out", "w", stdout);
  }
  int testcase = 1;
  // cin >> testcase;
  while (testcase--)
    SaKaTa();
  return 0;
}
