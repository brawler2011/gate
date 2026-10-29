import {describe, expect, test} from "bun:test";

import {loadTestState, renumberTests, serializeTestState} from "../components/workshop/tests/testsConfig";

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
