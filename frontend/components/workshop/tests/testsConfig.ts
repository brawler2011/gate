import {formatPaddedOrdinal} from "./types";

import type {SubtaskItem, TestItem} from "./types";

type TestsConfig = { groups?: Record<string, unknown>[]; tests?: Record<string, unknown>[] };

export const loadTestState = (
  configData: TestsConfig, existingFiles: string[], fileSizeMap = new Map<string, number>()
): {loadedSubtasks: SubtaskItem[]; loadedTests: TestItem[]} => {
  const rawGroups = configData.groups || [];
  const rawTests = configData.tests || [];

  // The manifest defines membership; unrelated files are not tests.
  const ordinalsSet = new Set<number>();
  for (const rt of rawTests) {
    if (typeof rt.ordinal === "number") {
      ordinalsSet.add(rt.ordinal);
    }
  }

  const sortedOrdinals = Array.from(ordinalsSet).sort((a, b) => a - b);

  // Build SubtaskItems
  const loadedSubtasks: SubtaskItem[] = rawGroups.map((grp: Record<string, unknown>, idx: number) => {
    const testsArr = grp.tests as number[] | undefined;
    const start = testsArr?.[0] ?? 1;
    const end = testsArr?.[1] ?? 0;
    const testIds: string[] = [];

    for (let ord = start; ord <= end; ord++) {
      testIds.push(`test-id-${ord}`);
    }

    const dependsArr = grp.depends_on as unknown[] | undefined;

    return {
      name: (grp.name as string) || `subtask${idx + 1}`,
      points: (grp.points as number) ?? 10,
      policy: grp.points_policy === "each" || grp.points_policy === "each-test" || grp.policy === "each" ? "each" : "complete",
      dependencies: dependsArr ? dependsArr.map((d: unknown) => (typeof d === "string" ? d : (rawGroups.find((g) => g.ordinal === d)?.name as string) || `subtask${d}`)) : [],
      testIds,
    };
  });

  // Build TestItems
  const loadedTests: TestItem[] = sortedOrdinals.map((ord) => {
    const padded = formatPaddedOrdinal(ord);
    const rawT = rawTests.find((t: Record<string, unknown>) => t.ordinal === ord);
    const declaredName = (rawT?.filename as string) || `${padded}.in`;
    const stem = declaredName.replace(/\.[^/.]+$/, "");
    const inName = [declaredName, `${stem}.in`, `${stem}.input`]
      .find((name) => existingFiles.includes(name)) || declaredName;
    const outName = [stem, declaredName].flatMap((name) =>
      ["out", "ans", "a", "output"].map((ext) => `${name}.${ext}`))
      .find((name) => existingFiles.includes(name)) || `${stem}.out`;

    const hasIn = existingFiles.includes(inName);
    const hasOut = existingFiles.includes(outName);

    const subtaskNames = loadedSubtasks
      .filter((st) => st.testIds.includes(`test-id-${ord}`))
      .map((st) => st.name);

    return {
      id: `test-id-${ord}`,
      ordinal: ord,
      filename: inName,
      manifestFilename: (rawT?.filename as string) || "",
      outFilename: outName,
      hasIn,
      hasOut,
      inSize: fileSizeMap.get(inName),
      outSize: fileSizeMap.get(outName),
      isSample: !!rawT?.is_sample,
      method: rawT?.method === "generated" ? "generated" : "manual",
      generatorCommand: (rawT?.generator as string) || "",
      subtaskNames,
    };
  });

  return {loadedSubtasks, loadedTests};
};

  type PayloadGroup = {
    ordinal: number;
    name: string;
    points: number;
    points_policy: string;
    depends_on: number[];
    tests: number[];
  };

type PayloadTest = {
 ordinal: number; filename: string; method: "manual" | "generated"; generator: string | null; is_sample: boolean;
};

export const serializeTestState = (
  tests: TestItem[], subtasks: SubtaskItem[], problemType: string, maxScore: number | null
): {groups: PayloadGroup[]; tests: PayloadTest[]} => {

  let payloadGroups: PayloadGroup[] = subtasks
    .map((st, idx): PayloadGroup => {
      const stTests = tests.filter(
        (t) => st.testIds.includes(t.id) || t.subtaskNames.includes(st.name)
      );
      const ordinals = stTests.map((t) => t.ordinal).sort((a, b) => a - b);

      return {
        ordinal: idx + 1,
        name: st.name,
        points: st.points,
        points_policy: st.policy,
        depends_on: st.dependencies.map((name) => subtasks.findIndex((group) => group.name === name) + 1),
        tests: ordinals.length ? [ordinals[0], ordinals[ordinals.length - 1]] : [1, 0],
      };
    });

  // If subtasks is empty or no subtask covers tests, but tests exist, create a default subtask covering all tests
  if (payloadGroups.length === 0 && tests.length > 0) {
    const sortedOrds = tests.map((t) => t.ordinal).sort((a, b) => a - b);
    payloadGroups = [
      {
        ordinal: 1,
        name: "subtask1",
        points: maxScore ?? 100,
        points_policy: problemType === "scoring" ? "each" : "complete",
        depends_on: [],
        tests: [sortedOrds[0], sortedOrds[sortedOrds.length - 1]],
      },
    ];
  }

  // Ensure all tests are covered by groups so backend problem.yaml doesn't drop unassigned tests
  if (payloadGroups.length > 0 && tests.length > 0) {
    const covered = new Set<number>();
    for (const g of payloadGroups) {
      for (let ord = g.tests[0]; ord <= g.tests[1]; ord++) {
        covered.add(ord);
      }
    }

    const unassigned = tests
      .map((t) => t.ordinal)
      .filter((ord) => !covered.has(ord))
      .sort((a, b) => a - b);

    if (unassigned.length > 0) {
      const lastGroup = payloadGroups[payloadGroups.length - 1];
      lastGroup.tests[0] = Math.min(lastGroup.tests[0], unassigned[0]);
      lastGroup.tests[1] = Math.max(lastGroup.tests[1], unassigned[unassigned.length - 1]);
    }
  }

  const payloadTests = tests.map((t) => ({
    ordinal: t.ordinal,
    filename: t.method === "manual" ? t.manifestFilename || t.filename : "",
    method: t.method,
    generator: t.method === "generated" && t.generatorCommand ? t.generatorCommand : null,
    is_sample: t.isSample,
  }));

  return {
    groups: payloadGroups,
    tests: payloadTests,
  };

};

// Ordinals are presentation order; filenames and in-memory IDs retain identity.
export const renumberTests = (tests: TestItem[]): TestItem[] => {
  return tests.map((test, index) => ({...test, ordinal: index + 1}));
};
