#include "testlib.h"
#include <iostream>
#include <vector>

using namespace std;

int main(int argc, char* argv[]) {
    registerGen(argc, argv, 1);
    
    int n = atoi(argv[1]);
    int limit = atoi(argv[2]); // elements between -limit and limit
    
    cout << n << endl;
    for (int i = 0; i < n; i++) {
        int val = rnd.next(-limit, limit);
        cout << val << (i == n - 1 ? "" : " ");
    }
    cout << endl;
    return 0;
}
