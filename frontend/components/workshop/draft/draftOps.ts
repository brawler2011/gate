import type {
  ProblemDraft,
  ProblemDraftLibraryFile,
  ProblemDraftNamedProgram,
  ProblemDraftProgram,
  ProblemDraftStatement,
  ProblemDraftSubtask,
  ProblemDraftTest,
  SupportedLanguage,
  SystemLibraryFile,
} from "@/contracts/core/v1/schema";

/**
 * Edit operations of the workshop. They only change the draft; the server
 * validates the result on save and names the offending entry.
 */

export type ComponentRole = "checker" | "validator" | "interactor" | "generator" | "solution";

export const ROLE_LABELS: Record<ComponentRole, string> = {
  checker: "Чекер",
  validator: "Валидатор",
  interactor: "Интерактор",
  generator: "Генератор",
  solution: "Решение",
};

/** Roles the draft holds as a single program field rather than a list. */
export type SingleRole = "checker" | "interactor";
export type ListRole = "validator" | "generator" | "solution";

export const isSingleRole = (role: ComponentRole): role is SingleRole => role === "checker" || role === "interactor";

const LIST_FIELDS = {validator: "validators", generator: "generators", solution: "solutions"} as const;

export type OpResult = {draft: ProblemDraft} | {error: string};

const newId = (): string => crypto.randomUUID();

/** Points are exact hundredths; arithmetic runs on integers. */
export const toHundredths = (points: number): number => Math.round(points * 100);
const fromHundredths = (hundredths: number): number => hundredths / 100;
export const formatPoints = (points: number): string => points.toFixed(2).replace(/\.?0+$/, "");

export const sumPoints = (tests: ProblemDraftTest[]): number =>
  fromHundredths(tests.reduce((total, test) => total + toHundredths(test.points), 0));

export const programsOf = (draft: ProblemDraft, role: ListRole): ProblemDraftNamedProgram[] => draft[LIST_FIELDS[role]];

/** Sets the single checker or interactor; null removes it. */
export const setSingleProgram = (
  draft: ProblemDraft,
  role: SingleRole,
  program: ProblemDraftProgram | null,
): ProblemDraft => ({
  ...draft,
  [role]: program,
});

export const addProgram = (
  draft: ProblemDraft,
  role: ListRole,
  name: string,
  language: string,
  source: string,
  dependencies: string[] = [],
): ProblemDraft => {
  const program: ProblemDraftNamedProgram = {id: newId(), name, language, source, dependencies};
  return {
    ...draft,
    [LIST_FIELDS[role]]: [...programsOf(draft, role), program],
    // The first solution becomes the main one.
    main_solution_id: role === "solution" && !draft.main_solution_id ? program.id : draft.main_solution_id,
  };
};

export const updateProgram = (
  draft: ProblemDraft,
  role: ListRole,
  id: string,
  patch: Partial<Omit<ProblemDraftNamedProgram, "id">>,
): ProblemDraft => ({
  ...draft,
  [LIST_FIELDS[role]]: programsOf(draft, role).map((p) => (p.id === id ? {...p, ...patch} : p)),
});

export const removeProgram = (draft: ProblemDraft, role: ListRole, id: string): ProblemDraft => ({
  ...draft,
  [LIST_FIELDS[role]]: programsOf(draft, role).filter((p) => p.id !== id),
  main_solution_id: draft.main_solution_id === id ? null : draft.main_solution_id,
});

export const addLibraryFile = (draft: ProblemDraft, name: string): {draft: ProblemDraft; id: string} => {
  const file: ProblemDraftLibraryFile = {id: newId(), name, content: "", auto_update: false};
  return {draft: {...draft, library: [...draft.library, file]}, id: file.id};
};

/** Adds the system library file `name`, which every build takes at its current version. */
export const addSystemFile = (draft: ProblemDraft, name: string): {draft: ProblemDraft; id: string} => {
  const file: ProblemDraftLibraryFile = {id: newId(), name, content: "", auto_update: true};
  return {draft: {...draft, library: [...draft.library, file]}, id: file.id};
};

/** Turns an auto-updated file into an editable copy of the system text. */
export const stopAutoUpdate = (draft: ProblemDraft, id: string, systemText: string): ProblemDraft =>
  updateLibraryFile(draft, id, {content: systemText, auto_update: false});

/** A new program depends on the auto-updated files of the draft that the system library provides for its language. */
export const systemDependencies = (
  draft: ProblemDraft,
  system: SystemLibraryFile[],
  language: SupportedLanguage | undefined,
): string[] =>
  draft.library
    .filter((f) => f.auto_update && system.some((s) =>
      s.name === f.name && s.extensions.some((extension) => language?.extensions.includes(extension))))
    .map((f) => f.id);

export const updateLibraryFile = (
  draft: ProblemDraft,
  id: string,
  patch: Partial<Omit<ProblemDraftLibraryFile, "id">>,
): ProblemDraft => ({...draft, library: draft.library.map((f) => (f.id === id ? {...f, ...patch} : f))});

export const removeLibraryFile = (draft: ProblemDraft, id: string): ProblemDraft => ({
  ...draft,
  library: draft.library.filter((f) => f.id !== id),
});

export const STATEMENT_SECTIONS = ["legend", "input", "output", "interaction", "notes", "scoring"] as const;
export type StatementSection = (typeof STATEMENT_SECTIONS)[number];

export const emptyStatement = (language: string): ProblemDraftStatement => ({
  language,
  legend: "",
  input: "",
  output: "",
  notes: "",
  interaction: "",
  scoring: "",
});

/** Replaces the statement of `statement.language`, adding it when missing. */
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

/** Removes a subtask and the dependencies other subtasks have on it. */
export const removeSubtask = (draft: ProblemDraft, id: string): ProblemDraft => ({
  ...draft,
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

/** Closes gaps so test ordinals are exactly 1..n in their current order. */
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

/** Moves a test by `delta` positions, keeping ordinals exactly 1..n. */
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
    ? {mode: "interactive", object_id: null, solution_id: null}
    : {mode, object_id: null, solution_id: mode === "generated" ? draft.main_solution_id : null};

export const newGeneratedTest = (draft: ProblemDraft, subtaskId: string): ProblemDraftTest => ({
  id: newId(),
  ordinal: nextOrdinal(draft.tests),
  subtask_id: subtaskId,
  sample: false,
  points: 0,
  input: {mode: "generator", object_id: null, generator_id: draft.generators[0]?.id ?? null, args: [], seed: 1},
  answer: newAnswer(draft, "generated"),
});

export const newManualTest = (draft: ProblemDraft, subtaskId: string): ProblemDraftTest => ({
  id: newId(),
  ordinal: nextOrdinal(draft.tests),
  subtask_id: subtaskId,
  sample: false,
  points: 0,
  input: {mode: "manual", object_id: null, generator_id: null, args: [], seed: null},
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

/**
 * Turns a JSON Pointer the server reported into a label of the draft entry it
 * points at, for example "Генератор gen" or "Тест 4".
 */
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
      return `Подзадача ${draft.subtasks[at]?.name ?? ""}`.trim();
    case "tests":
      return draft.tests[at] ? `Тест ${draft.tests[at].ordinal}` : "Тесты";
    case "main_solution_id":
      return "Главное решение";
    default:
      return "Черновик";
  }
};
