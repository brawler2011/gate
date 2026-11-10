import type {ComponentRole} from "./draftOps";

const TESTLIB = '#include "testlib.h"\n\n';

const CPP: Record<ComponentRole, string> = {
  checker: `${TESTLIB}int main(int argc, char* argv[]) {\n    setName("checker");\n    registerTestlibCmd(argc, argv);\n\n    quitf(_ok, "correct");\n}\n`,
  validator: `${TESTLIB}int main(int argc, char* argv[]) {\n    registerValidation(argc, argv);\n\n    inf.readEof();\n    return 0;\n}\n`,
  interactor: `${TESTLIB}int main(int argc, char* argv[]) {\n    setName("interactor");\n    registerInteraction(argc, argv);\n\n    quitf(_ok, "solved");\n}\n`,
  generator: `${TESTLIB}int main(int argc, char* argv[]) {\n    registerGen(argc, argv, 1);\n\n    return 0;\n}\n`,
  solution: "#include <iostream>\n\nint main() {\n    long long a, b;\n    std::cin >> a >> b;\n    std::cout << a + b << '\\n';\n}\n",
};

const PYTHON: Record<ComponentRole, string> = {
  checker: "import sys\n\n# argv: input, output, answer\nsys.exit(0)\n",
  validator: "import sys\n\ndata = sys.stdin.read()\n",
  interactor: "import sys\n",
  generator: "import sys\n\n# arguments are in sys.argv[1:]\n",
  solution: "a, b = map(int, input().split())\nprint(a + b)\n",
};

/** Returns a starter source for a new component; unknown languages start empty. */
export const componentTemplate = (role: ComponentRole, language: string): string => {
  if (language === "cpp") {
    return CPP[role];
  }
  if (language === "python") {
    return PYTHON[role];
  }
  return "";
};
