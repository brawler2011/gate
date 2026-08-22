#include "testlib.h"
#include <iostream>
#include <string>

using namespace std;

int main(int argc, char* argv[]) {
    registerInteraction(argc, argv);
    
    // Read bounds from the test input (inf)
    int n = inf.readInt(1, 1000000000, "n");
    int x = inf.readInt(1, n, "x");
    
    // Print upper bound N to contestant
    cout << n << endl;
    
    int queries = 0;
    while (queries < 30) {
        string type = ouf.readToken("(\\?|!)", "query_type");
        int y = ouf.readInt(1, n, "y");
        
        if (type == "!") {
            if (y == x) {
                quitf(_ok, "Correct answer: %d", x);
            } else {
                quitf(_wa, "Wrong answer: expected %d, found %d", x, y);
            }
        } else {
            queries++;
            if (y < x) {
                cout << ">" << endl;
            } else if (y > x) {
                cout << "<" << endl;
            } else {
                cout << "=" << endl;
            }
        }
    }
    
    quitf(_wa, "Too many queries (limit 30)");
    return 0;
}
