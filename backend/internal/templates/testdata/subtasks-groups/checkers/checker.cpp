#include "testlib.h"

int main(int argc, char* argv[]) {
    registerTestlibCmd(argc, argv);
    
    std::string jans = ans.readWord();
    std::string pans = ouf.readWord();
    
    if (jans != pans) {
        quitf(_wa, "expected %s, found %s", jans.c_str(), pans.c_str());
    }
    
    quitf(_ok, "correct parity");
    return 0;
}
