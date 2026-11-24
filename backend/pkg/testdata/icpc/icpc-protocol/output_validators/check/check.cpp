#include <cstdio>
#include <string>

int main(int argc, char** argv) {
    if (argc < 4) return 1;
    long long expected, got;
    FILE* answer = std::fopen(argv[2], "r");
    if (!answer || std::fscanf(answer, "%lld", &expected) != 1) return 1;
    bool ok = std::scanf("%lld", &got) == 1;
    FILE* message = std::fopen((std::string(argv[3]) + "/judgemessage.txt").c_str(), "w");
    if (!ok || got != expected) {
        std::fprintf(message, "expected %lld\n", expected);
        return 43;
    }
    return 42;
}
