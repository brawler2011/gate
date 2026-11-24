// The default output validator of the ICPC problem package format: input, answer and feedback
// directory as arguments followed by validator flags, team output on standard input.
#include <cctype>
#include <cmath>
#include <cstdlib>
#include <fstream>
#include <iostream>
#include <iterator>
#include <string>

static std::string feedback;

[[noreturn]] static void verdict(int code, const std::string& message) {
    if (!message.empty()) {
        std::ofstream(feedback + "judgemessage.txt") << message << '\n';
    }
    std::exit(code);
}

struct Tokens {
    std::string text;
    size_t pos = 0;
    // Returns false at the end; space receives the whitespace before the token.
    bool next(std::string& space, std::string& token) {
        size_t start = pos;
        while (pos < text.size() && std::isspace(static_cast<unsigned char>(text[pos]))) pos++;
        space.assign(text, start, pos - start);
        start = pos;
        while (pos < text.size() && !std::isspace(static_cast<unsigned char>(text[pos]))) pos++;
        token.assign(text, start, pos - start);
        return !token.empty();
    }
};

// Infinities and NaNs compare as text, as in the reference validator.
static bool number(const std::string& s, long double& value) {
    char* end = nullptr;
    value = std::strtold(s.c_str(), &end);
    return !s.empty() && *end == '\0' && std::isfinite(value);
}

static std::string lower(std::string s) {
    for (char& c : s) c = static_cast<char>(std::tolower(static_cast<unsigned char>(c)));
    return s;
}

int main(int argc, char** argv) {
    if (argc < 4) {
        std::cerr << "usage: " << argv[0] << " input answer feedback_dir [flags]\n";
        return 1;
    }
    feedback = argv[3];
    bool caseSensitive = false, spaceSensitive = false;
    long double absolute = -1, relative = -1;
    for (int i = 4; i < argc; i++) {
        std::string flag = argv[i];
        if (flag == "case_sensitive") {
            caseSensitive = true;
        } else if (flag == "space_change_sensitive") {
            spaceSensitive = true;
        } else if (i + 1 < argc && (flag == "float_tolerance" || flag == "float_absolute_tolerance" || flag == "float_relative_tolerance")) {
            long double tolerance = std::strtold(argv[++i], nullptr);
            if (flag != "float_relative_tolerance") absolute = tolerance;
            if (flag != "float_absolute_tolerance") relative = tolerance;
        } else {
            std::cerr << "unknown validator flag " << flag << '\n';
            return 1;
        }
    }
    std::ifstream answer(argv[2], std::ios::binary);
    if (!answer) {
        std::cerr << "cannot open answer " << argv[2] << '\n';
        return 1;
    }
    Tokens judge{std::string(std::istreambuf_iterator<char>(answer), {})};
    Tokens team{std::string(std::istreambuf_iterator<char>(std::cin), {})};
    std::string judgeSpace, judgeToken, teamSpace, teamToken;
    for (int n = 1;; n++) {
        bool more = judge.next(judgeSpace, judgeToken), teamMore = team.next(teamSpace, teamToken);
        std::string at = "token " + std::to_string(n) + ": ";
        if (spaceSensitive && judgeSpace != teamSpace) verdict(43, at + "whitespace differs");
        if (!more && !teamMore) verdict(42, "");
        if (!more) verdict(43, at + "trailing output " + teamToken);
        if (!teamMore) verdict(43, at + "missing output, expected " + judgeToken);
        long double expected, got;
        if ((absolute >= 0 || relative >= 0) && number(judgeToken, expected)) {
            if (!number(teamToken, got)) verdict(43, at + "expected a number, got " + teamToken);
            long double diff = std::fabs(expected - got);
            if (!(diff <= absolute || diff <= relative * std::fabs(expected))) {
                verdict(43, at + "expected " + judgeToken + ", got " + teamToken);
            }
        } else if (caseSensitive ? judgeToken != teamToken : lower(judgeToken) != lower(teamToken)) {
            verdict(43, at + "expected " + judgeToken + ", got " + teamToken);
        }
    }
}
