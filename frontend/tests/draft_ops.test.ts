import {describe, expect, test} from "bun:test";

import {
  addComponent,
  addLibraryFile,
  addSubtask,
  buildProblems,
  formatPoints,
  libraryFiles,
  newGeneratedTest,
  moveTest,
  newManualTest,
  removeComponent,
  setComponentLanguage,
  removeSubtask,
  removeTest,
  setSubtaskDependencies,
  sumPoints,
  upsertTest,
} from "@/components/workshop/draft/draftOps";

import type {ProblemDraft} from "@/contracts/core/v1/schema";

const emptyDraft = (): ProblemDraft => ({
  problem_id: "11111111-1111-4111-8111-111111111111",
  revision: 1,
  title: "A",
  short_name: "a",
  type: "pass-fail",
  time_limit_ms: 1000,
  memory_limit_mb: 256,
  max_score: null,
  main_solution_id: null,
  files: {},
  components: [],
  subtasks: [],
  tests: [],
  assets: [],
});

const must = (result: ReturnType<typeof addComponent>): ProblemDraft => {
  if ("error" in result) {
    throw new Error(result.error);
  }
  return result.draft;
};

describe("components", () => {
  test("checker, validator and interactor are singletons; generators and solutions are not", () => {
    for (const role of ["checker", "validator", "interactor"] as const) {
      const draft = must(addComponent(emptyDraft(), role, "main", "cpp", "int main(){}"));
      expect(draft.components).toHaveLength(1);
      expect("error" in addComponent(draft, role, "other", "python", "")).toBe(true);
    }
    const draft = must(addComponent(emptyDraft(), "checker", "check", "cpp", "int main(){}"));
    expect(draft.files["checkers/check.cpp"]).toBe("int main(){}");
    const generators = must(addComponent(must(addComponent(emptyDraft(), "generator", "a", "cpp", "")), "generator", "b", "python", ""));
    expect(generators.components.map((c) => c.path)).toEqual(["generators/a.cpp", "generators/b.py"]);
  });

  test("rejects duplicate paths, path traversal and unknown languages", () => {
    const draft = must(addComponent(emptyDraft(), "generator", "gen", "python", ""));
    expect("error" in addComponent(draft, "generator", "gen", "python", "")).toBe(true);
    expect("error" in addComponent(draft, "generator", "../evil", "python", "")).toBe(true);
    expect("error" in addComponent(draft, "generator", "a/b", "python", "")).toBe(true);
    expect("error" in addComponent(draft, "generator", "gen", "rust", "")).toBe(true);
  });

  test("changing the language renames the source and keeps the component", () => {
    let draft = must(addComponent(emptyDraft(), "checker", "checker", "cpp", "src"));
    const checker = draft.components[0];
    draft = must(addComponent(draft, "generator", "gen", "cpp", ""));
    draft = {...draft, files: {...draft.files, "lib/testlib.h": ""}};
    draft.components[0] = {...checker, dependencies: {"testlib.h": "lib/testlib.h"}};
    draft = addSubtask(draft, "All");
    draft = upsertTest(draft, newGeneratedTest(draft, draft.subtasks[0].id));

    const renamed = must(setComponentLanguage(draft, checker.id, "python"));
    expect(renamed.components[0]).toEqual({...checker, path: "checkers/checker.py", dependencies: {"testlib.h": "lib/testlib.h"}});
    expect(renamed.files["checkers/checker.py"]).toBe("src");
    expect("checkers/checker.cpp" in renamed.files).toBe(false);
    expect(renamed.tests).toEqual(draft.tests);

    // A file named only by its extension is a Python source on the server too.
    const dotfile: ProblemDraft = {
      ...emptyDraft(),
      files: {"validators/.py": ""},
      components: [{id: "v", role: "validator", path: "validators/.py", dependencies: {}}],
    };
    expect(must(setComponentLanguage(dotfile, dotfile.components[0].id, "cpp")).components[0].path).toBe("validators/.cpp");

    const generator = draft.components[1].id;
    const python = must(setComponentLanguage(draft, generator, "python"));
    expect(python.components[1].path).toBe("generators/gen.py");
    expect(python.tests[0].input.component_id).toBe(generator);
  });

  test("removal clears the main solution and the source, but is refused while tests use it", () => {
    let draft = must(addComponent(emptyDraft(), "solution", "main", "cpp", ""));
    const solution = draft.components[0].id;
    expect(draft.main_solution_id).toBe(solution);
    draft = addSubtask(draft, "All");
    draft = upsertTest(draft, {
      ...newGeneratedTest(draft, draft.subtasks[0].id),
    });
    expect(draft.tests[0].answer.component_id).toBe(solution);
    expect("error" in removeComponent(draft, solution)).toBe(true);

    draft = removeTest(draft, draft.tests[0].id);
    const removed = must(removeComponent(draft, solution));
    expect(removed.main_solution_id).toBeNull();
    expect(removed.files["solutions/main.cpp"]).toBeUndefined();
  });
});

describe("library", () => {
  test("uses lib/, where templates and imports put shared files", () => {
    const draft = {...emptyDraft(), files: {"lib/testlib.h": "", "checkers/checker.cpp": ""}};
    expect(libraryFiles(draft)).toEqual(["lib/testlib.h"]);
    const result = addLibraryFile(draft, "olymp.sty", "");
    expect("draft" in result && libraryFiles(result.draft)).toEqual(["lib/olymp.sty", "lib/testlib.h"]);
  });
});

describe("subtasks and tests", () => {
  test("removing a test closes ordinal gaps", () => {
    let draft = addSubtask(emptyDraft(), "All");
    const subtask = draft.subtasks[0].id;
    for (let i = 0; i < 3; i++) {
      draft = upsertTest(draft, newManualTest(draft, subtask));
    }
    expect(draft.tests.map((t) => t.ordinal)).toEqual([1, 2, 3]);
    draft = removeTest(draft, draft.tests[0].id);
    expect(draft.tests.map((t) => t.ordinal)).toEqual([1, 2]);
  });

  test("moving a test swaps it with its neighbour and keeps ordinals dense", () => {
    let draft = addSubtask(emptyDraft(), "All");
    for (let i = 0; i < 3; i++) {
      draft = upsertTest(draft, {...newManualTest(draft, draft.subtasks[0].id), sample: i === 0});
    }
    const [first, second, third] = draft.tests.map((t) => t.id);
    draft = moveTest(draft, third, -1);
    expect(draft.tests.map((t) => [t.id, t.ordinal])).toEqual([[first, 1], [third, 2], [second, 3]]);
    // Moving past either end is a no-op rather than an error.
    expect(moveTest(draft, first, -1)).toBe(draft);
    expect(moveTest(draft, second, 1)).toBe(draft);
  });

  test("a subtask with tests cannot be deleted; deleting another drops dependencies on it", () => {
    let draft = addSubtask(addSubtask(emptyDraft(), "A"), "B");
    const [a, b] = draft.subtasks.map((s) => s.id);
    draft = must(setSubtaskDependencies(draft, b, [a]));
    draft = upsertTest(draft, newManualTest(draft, a));
    expect("error" in removeSubtask(draft, a)).toBe(true);
    const without = must(removeSubtask({...draft, tests: []}, a));
    expect(without.subtasks[0].depends_on).toEqual([]);
  });

  test("dependency cycles are rejected", () => {
    let draft = addSubtask(addSubtask(addSubtask(emptyDraft(), "A"), "B"), "C");
    const [a, b, c] = draft.subtasks.map((s) => s.id);
    draft = must(setSubtaskDependencies(draft, b, [a]));
    draft = must(setSubtaskDependencies(draft, c, [b]));
    expect("error" in setSubtaskDependencies(draft, a, [c])).toBe(true);
    expect("error" in setSubtaskDependencies(draft, a, [a])).toBe(true);
  });
});

describe("points", () => {
  test("weights are summed in exact hundredths", () => {
    const base = addSubtask(emptyDraft(), "A");
    const test = (points: number) => ({...newManualTest(base, base.subtasks[0].id), points});
    // Naive float addition gives 0.30000000000000004.
    expect(sumPoints([test(0.1), test(0.2)])).toBe(0.3);
    expect(sumPoints([test(33.33), test(33.33), test(33.34)])).toBe(100);
  });

  test("formats without trailing zeros", () => {
    expect(formatPoints(10)).toBe("10");
    expect(formatPoints(3.3)).toBe("3.3");
    expect(formatPoints(3.33)).toBe("3.33");
    expect(formatPoints(0)).toBe("0");
  });
});

describe("build readiness", () => {
  test("reports everything missing before a build is requested", () => {
    const draft = emptyDraft();
    expect(buildProblems(draft)).toEqual(["Нет чекера", "Нет ни одного теста", "Нет ни одной подзадачи"]);
    expect(buildProblems({...draft, type: "interactive"})).toContain("Для интерактивной задачи нужен интерактор");
    expect(buildProblems({...draft, type: "interactive"})).toContain("Для интерактивной задачи нужно главное решение");
  });

  test("manual tests need uploaded objects", () => {
    let draft = must(addComponent(emptyDraft(), "checker", "check", "cpp", ""));
    draft = addSubtask(draft, "All");
    draft = upsertTest(draft, newManualTest(draft, draft.subtasks[0].id));
    expect(buildProblems(draft)).toEqual(["У некоторых ручных тестов не загружен вход", "У некоторых тестов не загружен ответ"]);
  });
});
