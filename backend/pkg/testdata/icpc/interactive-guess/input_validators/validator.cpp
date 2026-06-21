#include "testlib.h"

int main(int argc, char* argv[]) {
    registerValidation(argc, argv);
    int n = inf.readInt(1, 1000000000, "n");
    inf.readSpace();
    inf.readInt(1, n, "x");
    inf.readEoln();
    inf.readEof();
    return 0;
}
