#include "testlib.h"

int main(int argc, char* argv[]) {
    registerTestlibCmd(argc, argv);
    // Standard checker for interactive task that passes if interactor succeeded
    quitf(_ok, "interactor verified success");
}
