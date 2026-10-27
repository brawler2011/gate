import {describe, expect, it} from "bun:test";

import {detectSubmissionLanguage} from "@/lib/detectSubmissionLanguage";

describe("submission language detection", () => {
  it.each([
    ["python", "def solve(n: int) -> int:\n    return n + 1"],
    ["python", "import sys\nn = int(input())\nprint(n)"],
    ["python", "for i in range(10):\n    print(i)"],
    ["cpp", "#include <bits/stdc++.h>\nint main() { return 0; }"],
    ["cpp", "# include <iostream>\nint main() {}"],
    ["cpp", "using namespace std;\nvoid solve() { vector<int> v; cout << 1; }"],
    ["golang", 'package main\nimport "fmt"\nfunc main() { fmt.Println(1) }'],
    ["golang", "func solve(n int) int { return n + 1 }"],
    ["golang", 'package main\nvar n = 1\nfunc main() {\n n := 2\n}'],
  ] as const)("detects %s source", (language, source) => {
    expect(detectSubmissionLanguage(source)).toBe(language);
  });

  it.each(["", "hello world", "x = 1", "print(1)", "int n;", "# def solve():\n# print(1)",
    "// func main() {}", '"def solve():"', '"""\ndef solve():\nprint(1)\n"""',
    "/* #include <iostream>\nint main() {} */",
    "def solve():\n  pass\nfunc main() {}",
  ])("does not guess for ambiguous text: %s", (source) => {
    expect(detectSubmissionLanguage(source)).toBeNull();
  });

  it("ignores misleading syntax inside strings and comments", () => {
    expect(detectSubmissionLanguage('package main\nfunc main() {\n // def solve():\n s := "#include <iostream>"\n}'))
      .toBe("golang");
  });

  it("bounds analysis of large sources", () => {
    expect(detectSubmissionLanguage(" ".repeat(32768) + "func main() {}" )).toBeNull();
    expect(detectSubmissionLanguage("def solve():\n pass\n" + " ".repeat(1_000_000))).toBe("python");
  });
});
