#include <cstdio>
#include <string>

// The range is the input; the hidden number is the judge answer.
int main(int argc, char** argv) {
    if (argc < 4) return 1;
    long long n, hidden;
    FILE* input = std::fopen(argv[1], "r");
    FILE* answer = std::fopen(argv[2], "r");
    if (!input || !answer || std::fscanf(input, "%lld", &n) != 1 || std::fscanf(answer, "%lld", &hidden) != 1) return 1;
    FILE* message = std::fopen((std::string(argv[3]) + "judgemessage.txt").c_str(), "w");
    std::printf("%lld\n", n);
    std::fflush(stdout);
    for (int queries = 0; queries <= 64; queries++) {
        char kind;
        long long x;
        if (std::scanf(" %c %lld", &kind, &x) != 2) {
            std::fprintf(message, "malformed query\n");
            return 43;
        }
        if (kind == '!') {
            if (x == hidden) return 42;
            std::fprintf(message, "answered %lld, hidden %lld\n", x, hidden);
            return 43;
        }
        std::printf("%c\n", hidden < x ? '<' : hidden > x ? '>' : '=');
        std::fflush(stdout);
    }
    std::fprintf(message, "too many queries\n");
    return 43;
}
