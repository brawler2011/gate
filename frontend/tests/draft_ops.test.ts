import {describe, expect, test} from "bun:test";

import {
  addGroup,
  addProgram,
  newGeneratedTest,
  describePointer,
  emptyStatement,
  formatPoints,
  groupMaxPoints,
  groupRows,
  moveTest,
  newManualTest,
  removeGroup,
  removeProgram,
  removeTest,
  renameProgram,
  requirableGroups,
  setValidatorGroups,
  sumPoints,
  updateGroup,
  upsertTest,
} from "@/components/workshop/draft/draftOps";

import type {OpResult} from "@/components/workshop/draft/draftOps";
import type {ProblemDraft} from "@/contracts/core/v1/schema";

const emptyDraft = (): ProblemDraft => ({
  problem_id: "11111111-1111-4111-8111-111111111111",
  revision: 1,
  title: "A",
  short_name: "a",
  type: "standard", grading: "pass-fail",
  time_limit_ms: 1000, time_rule: null,
  memory_limit_mb: 256, input_file: "", output_file: "", processes: 0,
  max_score: null,
  checker: null,
  interactor: null,
  validators: [],
  generators: [],
  solutions: [],
  main_solution: null,
  library: [], graders: [], solution_name: "", attachments: [],
  statements: [],
  assets: [],
  testset: {id: "root", name: "", aggregate: "sum", stop_on_failure: false, requires: [], feedback: "complete", checker_args: [], validator_args: [], groups: []},
  tests: [],
});

// groupIds lists the groups below the testset, parents first.
const groupIds = (draft: ProblemDraft): string[] => groupRows(draft).slice(1).map((r) => r.group.id);

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
    draft = upsertTest(draft, newGeneratedTest(draft, draft.testset.id));
    draft = upsertTest(draft, {...newGeneratedTest(draft, draft.testset.id), answer: {mode: "generated", object_id: null, solution: "wa"}});

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

describe("groups and tests", () => {
  test("removing a test closes ordinal gaps", () => {
    let draft = emptyDraft();
    for (let i = 0; i < 3; i++) {
      draft = upsertTest(draft, newManualTest(draft, draft.testset.id));
    }
    expect(draft.tests.map((t) => t.ordinal)).toEqual([1, 2, 3]);
    draft = removeTest(draft, draft.tests[0].id);
    expect(draft.tests.map((t) => t.ordinal)).toEqual([1, 2]);
  });

  test("moving a test swaps it with its neighbour and keeps ordinals dense", () => {
    let draft = emptyDraft();
    for (let i = 0; i < 3; i++) {
      draft = upsertTest(draft, {...newManualTest(draft, draft.testset.id), sample: i === 0});
    }
    const [first, second, third] = draft.tests.map((t) => t.id);
    draft = moveTest(draft, third, -1);
    expect(draft.tests.map((t) => [t.id, t.ordinal])).toEqual([[first, 1], [third, 2], [second, 3]]);
    // Moving past either end is a no-op rather than an error.
    expect(moveTest(draft, first, -1)).toBe(draft);
    expect(moveTest(draft, second, 1)).toBe(draft);
  });

  test("deleting a group hands its subtree's tests to its parent and drops requirements and validator scopes on it", () => {
    let draft = addGroup(addGroup(emptyDraft(), "root", "A"), "root", "B");
    const [a, b] = groupIds(draft);
    draft = addGroup(draft, a, "C");
    const c = groupIds(draft)[1];
    draft = upsertTest(updateGroup(draft, b, {requires: [c]}), newManualTest(draft, c));
    draft = setValidatorGroups(applied(addProgram(draft, "validator", "v", "cpp", "")), "v", [a, b]);
    const removed = removeGroup(draft, a);
    expect(removed.testset.groups.map((g) => [g.name, g.requires])).toEqual([["B", []]]);
    expect(removed.tests[0].group_id).toBe("root");
    expect(removed.validators[0].groups).toEqual([b]);
    expect(removeGroup(removed, b).validators[0].groups).toEqual([b]);
    expect(removeGroup(removed, "root")).toBe(removed);
  });

  test("a group may require neither itself, its ancestors nor its subgroups", () => {
    let draft = addGroup(addGroup(emptyDraft(), "root", "A"), "root", "B");
    draft = addGroup(draft, groupIds(draft)[0], "C");
    const [a, c, b] = groupIds(draft);
    expect(requirableGroups(draft, a).map((g) => g.id)).toEqual([b]);
    expect(requirableGroups(draft, c).map((g) => g.id)).toEqual([b]);
    expect(requirableGroups(draft, b).map((g) => g.id)).toEqual([a, c]);
  });
});

describe("points", () => {
  test("weights are summed in exact hundredths", () => {
    const base = emptyDraft();
    const test = (points: number) => ({...newManualTest(base, base.testset.id), points});
    // Point totals avoid floating-point rounding errors.
    expect(sumPoints([test(0.1), test(0.2)])).toBe(0.3);
    expect(sumPoints([test(33.33), test(33.33), test(33.34)])).toBe(100);
  });

  test("group maxima follow their aggregation, leaving out groups without tests", () => {
    let draft = addGroup(addGroup(addGroup(emptyDraft(), "root", "A"), "root", "B"), "root", "C");
    const [a, b, c] = groupIds(draft);
    draft = updateGroup(updateGroup(draft, b, {aggregate: "sum"}), c, {aggregate: "avg"});
    for (const [group, points] of [[a, 25], [a, 25], [b, 10], [b, 5.5]] as const) {
      draft = upsertTest(draft, {...newManualTest(draft, group), points});
    }
    expect([...groupMaxPoints(draft)]).toEqual([[a, 25], [b, 15.5], [c, 0], ["root", 40.5]]);
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
    draft = addGroup({...draft, statements: [emptyStatement("ru")]}, "root", "A");
    draft = upsertTest(draft, newManualTest(draft, groupIds(draft)[0]));
    expect(describePointer(draft, "/generators/0/source")).toBe("Генератор gen");
    expect(describePointer(draft, "/checker/language")).toBe("Чекер");
    expect(describePointer(draft, "/statements/0/legend")).toBe("Условие RU, легенда");
    expect(describePointer(draft, "/tests/0/input/object_id")).toBe("Тест 1");
    expect(describePointer(draft, "/testset/groups/0/requires/0")).toBe("Группа «A»");
    expect(describePointer(draft, "/testset/aggregate")).toBe("Группа «Все тесты»");
    // A pointer outside the sent draft still yields a readable label.
    expect(describePointer(draft, "/tests/9")).toBe("Тесты");
    expect(describePointer(draft, "")).toBe("Черновик");
  });
});
