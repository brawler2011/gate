import {describe, expect, test} from "bun:test";

import {
  addProgram,
  addSubtask,
  newGeneratedTest,
  describePointer,
  emptyStatement,
  formatPoints,
  moveTest,
  newManualTest,
  removeProgram,
  removeSubtask,
  removeTest,
  renameProgram,
  setSubtaskDependencies,
  sumPoints,
  upsertTest,
} from "@/components/workshop/draft/draftOps";

import type {OpResult} from "@/components/workshop/draft/draftOps";
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
  checker: null,
  interactor: null,
  validators: [],
  generators: [],
  solutions: [],
  main_solution: null,
  library: [],
  statements: [],
  assets: [],
  subtasks: [],
  tests: [],
});

const applied = (result: OpResult): ProblemDraft => {
  if ("error" in result) {
    throw new Error(result.error);
  }
  return result.draft;
};

describe("programs", () => {
  test("the first solution becomes the main one, and removing it clears the choice", () => {
    let draft = applied(addProgram(emptyDraft(), "solution", "ok", "cpp", ""));
    draft = applied(addProgram(draft, "solution", "wa", "python", ""));
    expect(draft.main_solution).toBe("ok");
    expect(removeProgram(draft, "solution", "wa").main_solution).toBe("ok");
    expect(removeProgram(draft, "solution", "ok").main_solution).toBeNull();
  });

  test("renaming a program carries the tests and the main solution that refer to it", () => {
    let draft = applied(addProgram(emptyDraft(), "generator", "gen", "cpp", ""));
    draft = applied(addProgram(draft, "solution", "ok", "cpp", ""));
    draft = applied(addProgram(draft, "solution", "wa", "cpp", ""));
    draft = addSubtask(draft, "All");
    draft = upsertTest(draft, newGeneratedTest(draft, draft.subtasks[0].id));
    draft = upsertTest(draft, {...newGeneratedTest(draft, draft.subtasks[0].id), answer: {mode: "generated", object_id: null, solution: "wa"}});

    draft = applied(renameProgram(draft, "generator", "gen", "random"));
    draft = applied(renameProgram(draft, "solution", "ok", "main"));
    expect(draft.main_solution).toBe("main");
    expect(draft.tests.map((t) => [t.input.generator, t.answer.solution])).toEqual([["random", "main"], ["random", "wa"]]);
    // A generator and a solution may share a name; a rename never touches the other role.
    draft = applied(renameProgram(applied(addProgram(draft, "generator", "wa", "cpp", "")), "generator", "wa", "big"));
    expect(draft.tests[1].answer.solution).toBe("wa");
  });

  test("a name taken in the role is refused", () => {
    const draft = applied(addProgram(applied(addProgram(emptyDraft(), "solution", "ok", "cpp", "")), "solution", "wa", "cpp", ""));
    expect(renameProgram(draft, "solution", "wa", "ok")).toHaveProperty("error");
    expect(addProgram(draft, "solution", "ok", "python", "")).toHaveProperty("error");
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

  test("deleting a subtask drops dependencies on it", () => {
    let draft = addSubtask(addSubtask(emptyDraft(), "A"), "B");
    const [a, b] = draft.subtasks.map((s) => s.id);
    draft = setSubtaskDependencies(draft, b, [a]);
    expect(removeSubtask(draft, a).subtasks).toEqual([{...draft.subtasks[1], depends_on: []}]);
  });
});

describe("points", () => {
  test("weights are summed in exact hundredths", () => {
    const base = addSubtask(emptyDraft(), "A");
    const test = (points: number) => ({...newManualTest(base, base.subtasks[0].id), points});
    // Point totals avoid floating-point rounding errors.
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

describe("server error pointers", () => {
  test("name the entry an error points at", () => {
    let draft = applied(addProgram(emptyDraft(), "generator", "gen", "cpp", ""));
    draft = addSubtask({...draft, statements: [emptyStatement("ru")]}, "A");
    draft = upsertTest(draft, newManualTest(draft, draft.subtasks[0].id));
    expect(describePointer(draft, "/generators/0/source")).toBe("Генератор gen");
    expect(describePointer(draft, "/checker/language")).toBe("Чекер");
    expect(describePointer(draft, "/statements/0/legend")).toBe("Условие RU, легенда");
    expect(describePointer(draft, "/tests/0/input/object_id")).toBe("Тест 1");
    expect(describePointer(draft, "/subtasks/0/depends_on/0")).toBe("Подзадача A");
    // A pointer outside the sent draft still yields a readable label.
    expect(describePointer(draft, "/tests/9")).toBe("Тесты");
    expect(describePointer(draft, "")).toBe("Черновик");
  });
});
