import type {
  ProblemDraft,
  ProblemDraftChecker,
  ProblemDraftCheckerTest,
  ProblemDraftLibraryFile,
  ProblemDraftNamedProgram,
  ProblemDraftSolution,
  ProblemDraftGroup,
  ProblemDraftStatement,
  ProblemDraftTest,
  ProblemDraftValidatorTest,
} from "@/contracts/core/v1/schema";

export type ComponentRole = "checker" | "validator" | "interactor" | "generator" | "solution";

export const ROLE_LABELS: Record<ComponentRole, string> = {
  checker: "Чекер",
  validator: "Валидатор",
  interactor: "Интерактор",
  generator: "Генератор",
  solution: "Решение",
};

export type SingleRole = "checker" | "interactor";
export type ListRole = "validator" | "generator" | "solution";

export const isSingleRole = (role: ComponentRole): role is SingleRole => role === "checker" || role === "interactor";

const LIST_FIELDS = {validator: "validators", generator: "generators", solution: "solutions"} as const;

export type OpResult = {draft: ProblemDraft} | {error: string};

const newId = (): string => crypto.randomUUID();

// Use integer hundredths to avoid floating-point errors in point totals.
export const toHundredths = (points: number): number => Math.round(points * 100);
const fromHundredths = (hundredths: number): number => hundredths / 100;
export const formatPoints = (points: number): string => points.toFixed(2).replace(/\.?0+$/, "");

export const sumPoints = (tests: ProblemDraftTest[]): number =>
  fromHundredths(tests.reduce((total, test) => total + toHundredths(test.points), 0));

export const programsOf = (draft: ProblemDraft, role: ListRole): ProblemDraftNamedProgram[] => draft[LIST_FIELDS[role]];

export const setSingleProgram = <Role extends SingleRole>(
  draft: ProblemDraft,
  role: Role,
  program: ProblemDraft[Role],
): ProblemDraft => ({
    ...draft,
    [role]: program,
  });

export const customChecker = (language: string, source: string): ProblemDraftChecker => ({language, source, protocol: "testlib", args: [], standard: "", tests: []});

export const standardChecker = (name: string, language: string): ProblemDraftChecker =>
  ({language, source: "", protocol: "testlib", args: [], standard: name, tests: []});

// Preserve system includes when converting a standard checker to editable source.
export const customizeChecker = (draft: ProblemDraft, source: string, systemFiles: string[]): ProblemDraft => {
  if (!draft.checker) {
    return draft;
  }
  const missing = systemFiles.filter(
    (name) => source.includes(`#include "${name}"`) && !draft.library.some((file) => file.name === name),
  );
  const next = {...draft, checker: {...customChecker(draft.checker.language, source), tests: draft.checker.tests}};
  return missing.reduce(addSystemFile, next);
};

const nameTaken = (role: ListRole, name: string): OpResult => ({
  error: `${ROLE_LABELS[role]} с именем «${name}» уже есть`,
});

export const addProgram = (
  draft: ProblemDraft,
  role: ListRole,
  name: string,
  language: string,
  source: string,
): OpResult => {
  if (programsOf(draft, role).some((p) => p.name === name)) {
    return nameTaken(role, name);
  }
  const extra = {validator: {protocol: "testlib", args: [], groups: [], tests: []}, solution: {verdicts: ["OK"], points: null}, generator: {}}[role];
  const program = {name, language, source, ...extra};
  return {
    draft: {
      ...draft,
      [LIST_FIELDS[role]]: [...programsOf(draft, role), program],
      main_solution: role === "solution" && !draft.main_solution ? name : draft.main_solution,
    },
  };
};

export const updateProgram = (
  draft: ProblemDraft,
  role: ListRole,
  name: string,
  patch: Partial<Omit<ProblemDraftNamedProgram, "name">>,
): ProblemDraft => ({
  ...draft,
  [LIST_FIELDS[role]]: programsOf(draft, role).map((p) => (p.name === name ? {...p, ...patch} : p)),
});

export const renameProgram = (draft: ProblemDraft, role: ListRole, from: string, to: string): OpResult => {
  if (programsOf(draft, role).some((p) => p.name === to)) {
    return nameTaken(role, to);
  }
  const follow = (name: string | null): string | null => (name === from ? to : name);
  return {
    draft: {
      ...draft,
      [LIST_FIELDS[role]]: programsOf(draft, role).map((p) => (p.name === from ? {...p, name: to} : p)),
      main_solution: role === "solution" ? follow(draft.main_solution) : draft.main_solution,
      tests: draft.tests.map((test) => ({
        ...test,
        input: role === "generator" ? {...test.input, generator: follow(test.input.generator)} : test.input,
        answer: role === "solution" ? {...test.answer, solution: follow(test.answer.solution)} : test.answer,
      })),
    },
  };
};

export const setSolutionExpectation = (
  draft: ProblemDraft,
  name: string,
  patch: Partial<Pick<ProblemDraftSolution, "verdicts" | "points">>,
): ProblemDraft => ({
  ...draft,
  solutions: draft.solutions.map((s) => (s.name === name ? {...s, ...patch} : s)),
});

const mapCheckerTests = (
  draft: ProblemDraft,
  change: (tests: ProblemDraftCheckerTest[]) => ProblemDraftCheckerTest[],
): ProblemDraft =>
  draft.checker ? {...draft, checker: {...draft.checker, tests: change(draft.checker.tests)}} : draft;

export const updateCheckerTest = (
  draft: ProblemDraft,
  id: string,
  patch: Partial<ProblemDraftCheckerTest>,
): ProblemDraft =>
  mapCheckerTests(draft, (tests) => tests.map((t) => (t.id === id ? {...t, ...patch} : t)));

export const removeCheckerTest = (draft: ProblemDraft, id: string): ProblemDraft =>
  mapCheckerTests(draft, (tests) => tests.filter((t) => t.id !== id));

const mapValidatorTests = (
  draft: ProblemDraft,
  validator: string,
  change: (tests: ProblemDraftValidatorTest[]) => ProblemDraftValidatorTest[],
): ProblemDraft => ({
  ...draft,
  validators: draft.validators.map((v) => (v.name === validator ? {...v, tests: change(v.tests)} : v)),
});

export const updateValidatorTest = (
  draft: ProblemDraft,
  validator: string,
  id: string,
  patch: Partial<ProblemDraftValidatorTest>,
): ProblemDraft =>
  mapValidatorTests(draft, validator, (tests) => tests.map((t) => (t.id === id ? {...t, ...patch} : t)));

export const removeValidatorTest = (draft: ProblemDraft, validator: string, id: string): ProblemDraft =>
  mapValidatorTests(draft, validator, (tests) => tests.filter((t) => t.id !== id));

export const removeProgram = (draft: ProblemDraft, role: ListRole, name: string): ProblemDraft => ({
  ...draft,
  [LIST_FIELDS[role]]: programsOf(draft, role).filter((p) => p.name !== name),
  main_solution: role === "solution" && draft.main_solution === name ? null : draft.main_solution,
});

export const addLibraryFile = (draft: ProblemDraft, name: string): ProblemDraft => ({
  ...draft,
  library: [...draft.library, {name, content: "", auto_update: false}],
});

export const addSystemFile = (draft: ProblemDraft, name: string): ProblemDraft => ({
  ...draft,
  library: [...draft.library, {name, content: "", auto_update: true}],
});

export const startAutoUpdate = (draft: ProblemDraft, name: string): ProblemDraft =>
  updateLibraryFile(draft, name, {content: "", auto_update: true});

export const stopAutoUpdate = (draft: ProblemDraft, name: string, systemText: string): ProblemDraft =>
  updateLibraryFile(draft, name, {content: systemText, auto_update: false});

export const updateLibraryFile = (
  draft: ProblemDraft,
  name: string,
  patch: Partial<ProblemDraftLibraryFile>,
): ProblemDraft => ({...draft, library: draft.library.map((f) => (f.name === name ? {...f, ...patch} : f))});

export const removeLibraryFile = (draft: ProblemDraft, name: string): ProblemDraft => ({
  ...draft,
  library: draft.library.filter((f) => f.name !== name),
});

export const STATEMENT_SECTIONS = ["legend", "input", "output", "interaction", "notes", "scoring", "tutorial"] as const;
export type StatementSection = (typeof STATEMENT_SECTIONS)[number];

export const emptyStatement = (language: string): ProblemDraftStatement => ({
  language,
  title: "",
  legend: "",
  input: "",
  output: "",
  notes: "",
  interaction: "",
  scoring: "",
  tutorial: "",
});

export const setStatement = (draft: ProblemDraft, statement: ProblemDraftStatement): ProblemDraft => ({
  ...draft,
  statements: draft.statements.some((s) => s.language === statement.language)
    ? draft.statements.map((s) => (s.language === statement.language ? statement : s))
    : [...draft.statements, statement],
});

export const renameAsset = (draft: ProblemDraft, from: string, to: string): ProblemDraft => ({
  ...draft,
  assets: draft.assets.map((a) => (a.name === from ? {...a, name: to} : a)),
});

export const removeAsset = (draft: ProblemDraft, name: string): ProblemDraft => ({
  ...draft,
  assets: draft.assets.filter((a) => a.name !== name),
});

export const groupLabel = (group: ProblemDraftGroup): string => group.name || "Все тесты";

export type GroupRow = {
  group: ProblemDraftGroup;
  depth: number;
  // IDs from the testset down to the group itself.
  path: string[];
};

export const groupRows = (draft: ProblemDraft): GroupRow[] => {
  const rows: GroupRow[] = [];
  const visit = (group: ProblemDraftGroup, path: string[]) => {
    const own = [...path, group.id];
    rows.push({group, depth: path.length, path: own});
    group.groups.forEach((child) => visit(child, own));
  };
  visit(draft.testset, []);
  return rows;
};

type GroupChange = (group: ProblemDraftGroup) => ProblemDraftGroup;

const mapGroup = (group: ProblemDraftGroup, id: string, change: GroupChange): ProblemDraftGroup =>
  group.id === id ? change(group) : {...group, groups: group.groups.map((child) => mapGroup(child, id, change))};

export const updateGroup = (draft: ProblemDraft, id: string, patch: Partial<Omit<ProblemDraftGroup, "id" | "groups">>): ProblemDraft => ({
  ...draft,
  testset: mapGroup(draft.testset, id, (g) => ({...g, ...patch})),
});

export const addGroup = (draft: ProblemDraft, parent: string, name: string): ProblemDraft => {
  const group: ProblemDraftGroup = {
    id: newId(),
    name,
    aggregate: "min",
    stop_on_failure: false,
    requires: [],
    feedback: "complete",
    checker_args: [],
    validator_args: [],
    groups: [],
  };
  return {...draft, testset: mapGroup(draft.testset, parent, (g) => ({...g, groups: [...g.groups, group]}))};
};

// Tests of a removed group and its subgroups move to its parent.
export const removeGroup = (draft: ProblemDraft, id: string): ProblemDraft => {
  const rows = groupRows(draft);
  const row = rows.find((r) => r.group.id === id);
  if (!row || row.depth === 0) {
    return draft;
  }
  const parent = row.path[row.path.length - 2];
  const removed = new Set(rows.filter((r) => r.path.includes(id)).map((r) => r.group.id));
  const prune = (group: ProblemDraftGroup): ProblemDraftGroup => ({
    ...group,
    requires: group.requires.filter((r) => !removed.has(r)),
    groups: group.groups.filter((child) => child.id !== id).map(prune),
  });
  return {
    ...draft,
    testset: prune(draft.testset),
    tests: draft.tests.map((t) => (removed.has(t.group_id) ? {...t, group_id: parent} : t)),
    // An emptied scope would mean every test, so keep it for the server to reject instead.
    validators: draft.validators.map((v) => {
      const groups = v.groups.filter((group) => !removed.has(group));
      return groups.length > 0 ? {...v, groups} : v;
    }),
  };
};

// A group may require any other group except its ancestors and subgroups.
export const requirableGroups = (draft: ProblemDraft, id: string): ProblemDraftGroup[] => {
  const rows = groupRows(draft);
  const own = rows.find((r) => r.group.id === id);
  return rows
    .filter((r) => r.depth > 0 && !own?.path.includes(r.group.id) && !r.path.includes(id))
    .map((r) => r.group);
};

// Mirrors the server: groups without tests are left out of their parent's aggregate.
export const groupMaxPoints = (draft: ProblemDraft): Map<string, number> => {
  const result = new Map<string, number>();
  const grade = (group: ProblemDraftGroup): number | null => {
    const scores = draft.tests.filter((t) => t.group_id === group.id).map((t) => toHundredths(t.points));
    for (const child of group.groups) {
      const score = grade(child);
      if (score !== null) {
        scores.push(score);
      }
    }
    if (scores.length === 0) {
      result.set(group.id, 0);
      return null;
    }
    const sum = scores.reduce((a, b) => a + b, 0);
    const average = Math.floor(sum / scores.length);
    const score = {sum, min: Math.min(...scores), max: Math.max(...scores), avg: average}[group.aggregate];
    result.set(group.id, fromHundredths(score));
    return score;
  };
  grade(draft.testset);
  return result;
};

export const setGroupTestPoints = (draft: ProblemDraft, id: string, points: number): ProblemDraft => ({
  ...draft,
  tests: draft.tests.map((t) => (t.group_id === id ? {...t, points} : t)),
});

export const setValidatorGroups = (draft: ProblemDraft, name: string, groups: string[]): ProblemDraft => ({
  ...draft,
  validators: draft.validators.map((v) => (v.name === name ? {...v, groups} : v)),
});

const nextOrdinal = (tests: ProblemDraftTest[]): number =>
  tests.reduce((max, test) => Math.max(max, test.ordinal), 0) + 1;

const renumberTests = (draft: ProblemDraft): ProblemDraft => ({
  ...draft,
  tests: [...draft.tests]
    .sort((a, b) => a.ordinal - b.ordinal)
    .map((test, index) => ({...test, ordinal: index + 1})),
});

export const upsertTest = (draft: ProblemDraft, test: ProblemDraftTest): ProblemDraft => {
  const exists = draft.tests.some((t) => t.id === test.id);
  const tests = exists ? draft.tests.map((t) => (t.id === test.id ? test : t)) : [...draft.tests, test];
  return {...draft, tests: [...tests].sort((a, b) => a.ordinal - b.ordinal)};
};

export const removeTest = (draft: ProblemDraft, id: string): ProblemDraft =>
  renumberTests({...draft, tests: draft.tests.filter((t) => t.id !== id)});

export const moveTest = (draft: ProblemDraft, id: string, delta: number): ProblemDraft => {
  const tests = [...draft.tests].sort((a, b) => a.ordinal - b.ordinal);
  const from = tests.findIndex((t) => t.id === id);
  const to = from + delta;
  if (from < 0 || to < 0 || to >= tests.length) {
    return draft;
  }
  const [moved] = tests.splice(from, 1);
  tests.splice(to, 0, moved);
  return {...draft, tests: tests.map((test, index) => ({...test, ordinal: index + 1}))};
};

const newAnswer = (draft: ProblemDraft, mode: "provided" | "generated"): ProblemDraftTest["answer"] =>
  draft.type === "interactive"
    ? {mode: "interactive", object_id: null, solution: null}
    : {mode, object_id: null, solution: mode === "generated" ? draft.main_solution : null};

export const newGeneratedTest = (draft: ProblemDraft, groupId: string): ProblemDraftTest => ({
  id: newId(),
  ordinal: nextOrdinal(draft.tests),
  group_id: groupId,
  sample: false,
  pretest: false,
  statement_input: null,
  statement_output: null,
  points: 0,
  input: {mode: "generator", object_id: null, generator: draft.generators[0]?.name ?? null, args: []},
  answer: newAnswer(draft, "generated"),
});

export const newManualTest = (draft: ProblemDraft, groupId: string): ProblemDraftTest => ({
  id: newId(),
  ordinal: nextOrdinal(draft.tests),
  group_id: groupId,
  sample: false,
  pretest: false,
  statement_input: null,
  statement_output: null,
  points: 0,
  input: {mode: "manual", object_id: null, generator: null, args: []},
  answer: newAnswer(draft, "provided"),
});

const SECTION_LABELS: Record<string, string> = {
  legend: "легенда",
  input: "формат входа",
  output: "формат выхода",
  notes: "примечания",
  interaction: "протокол",
  scoring: "оценивание",
  tutorial: "разбор",
};

const describeGroup = (draft: ProblemDraft, steps: string[]): string => {
  let group: ProblemDraftGroup | undefined = draft.testset;
  for (let i = 0; i + 1 < steps.length && steps[i] === "groups"; i += 2) {
    group = group?.groups[Number(steps[i + 1])];
  }
  return group ? `Группа «${groupLabel(group)}»` : "Группы";
};

export const describePointer = (draft: ProblemDraft, pointer: string): string => {
  const [field, index, sub] = pointer.split("/").slice(1);
  const at = Number(index);
  switch (field) {
    case "checker":
    case "interactor":
      return ROLE_LABELS[field];
    case "validators":
    case "generators":
    case "solutions": {
      const role = (Object.keys(LIST_FIELDS) as ListRole[]).find((r) => LIST_FIELDS[r] === field) as ListRole;
      const name = draft[field][at]?.name;
      return name ? `${ROLE_LABELS[role]} ${name}` : ROLE_LABELS[role];
    }
    case "library":
      return `Библиотека ${draft.library[at]?.name ?? ""}`.trim();
    case "statements": {
      const statement = draft.statements[at];
      const section = sub ? SECTION_LABELS[sub] ?? sub : "";
      return `Условие ${statement?.language.toUpperCase() ?? ""}${section ? `, ${section}` : ""}`.trim();
    }
    case "assets":
      return `Ресурс ${draft.assets[at]?.name ?? ""}`.trim();
    case "testset":
      return describeGroup(draft, pointer.split("/").slice(2));
    case "tests":
      return draft.tests[at] ? `Тест ${draft.tests[at].ordinal}` : "Тесты";
    case "main_solution":
      return "Главное решение";
    default:
      return "Черновик";
  }
};
