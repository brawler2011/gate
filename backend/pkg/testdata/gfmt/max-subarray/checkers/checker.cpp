#include "testlib.h"

int main(int argc, char* argv[]) {
    registerTestlibCmd(argc, argv);
    
    long long jans = ans.readLong();
    long long pans = ouf.readLong();
    
    if (jans != pans) {
        quitf(_wa, "expected %lld, found %lld", jans, pans);
    }
    
    quitf(_ok, "correct maximum sum");
    return 0;
}
