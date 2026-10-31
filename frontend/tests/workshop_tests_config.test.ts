import {describe, expect, test} from "bun:test";

import {allocateTestFiles, loadTestState, renumberTests, serializeTestState} from "../components/workshop/tests/testsConfig";

const group = (ordinal: number, name: string, tests: number[], depends_on: number[] = []) =>
  ({ordinal, name, tests, depends_on, points: 10, points_policy: "complete"});
const manual = (ordinal: number, filename: string) =>
  ({ordinal, filename, method: "manual" as const, generator: null, is_sample: false});

describe("workshop manifest identity", () => {
  for (const [filename, input, answer] of [
    ["edge", "edge.in", "edge.ans"],
    ["nested/edge.in", "nested/edge.in", "nested/edge.in.ans"],
  ]) {
    test(`resolves aliases without changing manifest reference ${filename}`, () => {
      const config = {groups: [group(1, "a", [1, 1])], tests: [manual(1, filename)]};
      const {loadedTests, loadedSubtasks} = loadTestState(config, [input, answer, "99.in"]);
      expect(loadedTests).toHaveLength(1);
      expect(loadedTests[0]).toMatchObject({filename: input, outFilename: answer, hasIn: true, hasOut: true});
      expect(serializeTestState(loadedTests, loadedSubtasks, "scoring", 100)).toEqual(config);
    });
  }

  test("preserves missing answers for reference solution evaluation", () => {
    const config = {groups: [group(1, "a", [1, 1])], tests: [manual(1, "edge.in")]};
    const {loadedTests, loadedSubtasks} = loadTestState(config, ["edge.in"]);
    expect(loadedTests[0].hasOut).toBe(false);
    expect(serializeTestState(loadedTests, loadedSubtasks, "scoring", 100)).toEqual(config);
  });

  test("preserves empty group membership and dependency names", () => {
    const config = {
      groups: [group(1, "a-empty", [1, 0]), group(2, "b", [1, 1]), group(3, "c", [2, 2], [1])],
      tests: [manual(1, "b.in"), manual(2, "c.in")],
    };
    const {loadedTests, loadedSubtasks} = loadTestState(config, ["b.in", "c.in"]);
    expect(loadedSubtasks[0].testIds).toEqual([]);
    expect(loadedSubtasks[2].dependencies).toEqual(["a-empty"]);
    expect(serializeTestState(loadedTests, loadedSubtasks, "scoring", 100)).toEqual(config);
  });

  test("renumbers presentation while retaining named assets and group membership", () => {
    const config = {
      groups: [group(1, "a", [1, 1]), group(2, "b", [3, 3], [1])],
      tests: [manual(1, "first.in"), manual(3, "last.in")],
    };
    const {loadedTests, loadedSubtasks} = loadTestState(config, ["first.in", "first.out", "last.in", "last.out"]);
    const renumbered = renumberTests(loadedTests);
    expect(renumbered[1]).toMatchObject({id: loadedTests[1].id, ordinal: 2, filename: "last.in", outFilename: "last.out"});
    const saved = serializeTestState(renumbered, loadedSubtasks, "scoring", 100);
    expect(saved.groups).toEqual([group(1, "a", [1, 1]), group(2, "b", [2, 2], [1])]);
    expect(saved.tests).toEqual([manual(1, "first.in"), manual(2, "last.in")]);
  });
});

test("allocates unused filenames after deleting and renumbering tests", () => {
  const files = new Map([
    ["01.in", "first"], ["01.out", "first answer"],
    ["02.in", "deleted"], ["02.out", "deleted answer"],
    ["03.in", "retained"], ["03.out", "retained answer"],
    ["04.ans", "orphan answer"],
  ]);
  const config = {groups: [group(1, "a", [1, 3])], tests: [manual(1, "01.in"), manual(2, "02.in"), manual(3, "03.in")]};
  const state = loadTestState(config, [...files.keys()]);
  const retained = renumberTests(state.loadedTests.filter((test) => test.ordinal !== 2));
  const allocated = allocateTestFiles(retained, [...files.keys()]);
  expect(allocated).toEqual({filename: "05.in", outFilename: "05.out"});
  expect(allocateTestFiles(retained, [])).toEqual({filename: "02.in", outFilename: "02.out"});
});

test("assigns an added test to a trailing empty group without duplicating earlier membership", () => {
  const config = {groups: [group(1, "a", [1, 1]), group(2, "z-empty", [2, 1])], tests: [manual(1, "a.in")]};
  const {loadedTests, loadedSubtasks} = loadTestState(config, ["a.in"]);
  const added = {...loadedTests[0], id: "new", ordinal: 2, filename: "new.in", manifestFilename: "new.in", subtaskNames: []};
  const saved = serializeTestState([...loadedTests, added], loadedSubtasks, "scoring", 100);
  expect(saved.groups).toEqual([group(1, "a", [1, 1]), group(2, "z-empty", [2, 2])]);
  expect(saved.tests.map((test) => test.filename)).toEqual(["a.in", "new.in"]);
});

test("deleting a middle test emits explicit membership without requiring renumbering", () => {
  const config = {
    groups: [group(1, "a", [1, 3]), group(2, "z", [4, 4], [1])],
    tests: [manual(1, "first.in"), manual(2, "middle.in"), manual(3, "last.in"), manual(4, "tail.in")],
  };
  const {loadedTests, loadedSubtasks} = loadTestState(config, ["first.in", "middle.in", "last.in", "tail.in", "02.in"]);
  const retained = loadedTests.filter((test) => test.ordinal !== 2);
  const saved = serializeTestState(retained, loadedSubtasks, "scoring", 100);
  expect(saved.tests).toEqual([manual(1, "first.in"), manual(3, "last.in"), manual(4, "tail.in")]);
  expect(saved.groups).toEqual(config.groups);
});
