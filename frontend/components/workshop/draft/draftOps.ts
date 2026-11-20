import type {
  ProblemDraft,
  ProblemDraftChecker,
  ProblemDraftCheckerTest,
  ProblemDraftLibraryFile,
  ProblemDraftNamedProgram,
  ProblemDraftSolution,
  ProblemDraftStatement,
  ProblemDraftSubtask,
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

export const customChecker = (language: string, source: string): ProblemDraftChecker => ({language, source, standard: "", tests: []});

export const standardChecker = (name: string, language: string): ProblemDraftChecker => ({language, source: "", standard: name, tests: []});

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
  const extra = {validator: {subtasks: [], tests: []}, solution: {verdicts: ["OK"], points: null}, generator: {}}[role];
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

export const STATEMENT_SECTIONS = ["legend", "input", "output", "interaction", "notes", "scoring"] as const;
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

export const subtaskLabel = (subtask: ProblemDraftSubtask): string => subtask.name || `Подзадача ${subtask.ordinal}`;

export const addSubtask = (draft: ProblemDraft, name: string): ProblemDraft => {
  const subtask: ProblemDraftSubtask = {
    id: newId(),
    ordinal: draft.subtasks.reduce((max, s) => Math.max(max, s.ordinal), 0) + 1,
    name,
    points: 0,
    policy: "complete",
    depends_on: [],
  };
  return {...draft, subtasks: [...draft.subtasks, subtask]};
};

export const setValidatorSubtasks = (draft: ProblemDraft, name: string, subtasks: string[]): ProblemDraft => ({
  ...draft,
  validators: draft.validators.map((v) => (v.name === name ? {...v, subtasks} : v)),
});

export const removeSubtask = (draft: ProblemDraft, id: string): ProblemDraft => ({
  ...draft,
  // An emptied scope would mean every test, so keep it for the server to reject instead.
  validators: draft.validators.map((v) => {
    const subtasks = v.subtasks.filter((subtask) => subtask !== id);
    return subtasks.length > 0 ? {...v, subtasks} : v;
  }),
  subtasks: draft.subtasks
    .filter((s) => s.id !== id)
    .map((s) => ({...s, depends_on: s.depends_on.filter((dependency) => dependency !== id)})),
});

export const setSubtaskDependencies = (draft: ProblemDraft, id: string, dependencies: string[]): ProblemDraft => ({
  ...draft,
  subtasks: draft.subtasks.map((s) => (s.id === id ? {...s, depends_on: dependencies} : s)),
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

export const newGeneratedTest = (draft: ProblemDraft, subtaskId: string): ProblemDraftTest => ({
  id: newId(),
  ordinal: nextOrdinal(draft.tests),
  subtask_id: subtaskId,
  sample: false,
  points: 0,
  input: {mode: "generator", object_id: null, generator: draft.generators[0]?.name ?? null, args: []},
  answer: newAnswer(draft, "generated"),
});

export const newManualTest = (draft: ProblemDraft, subtaskId: string): ProblemDraftTest => ({
  id: newId(),
  ordinal: nextOrdinal(draft.tests),
  subtask_id: subtaskId,
  sample: false,
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
    case "subtasks":
      return `Подзадача ${draft.subtasks[at]?.name || (draft.subtasks[at]?.ordinal ?? "")}`.trim();
    case "tests":
      return draft.tests[at] ? `Тест ${draft.tests[at].ordinal}` : "Тесты";
    case "main_solution":
      return "Главное решение";
    default:
      return "Черновик";
  }
};
