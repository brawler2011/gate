import type {
  ProblemDraft,
  ProblemDraftComponent,
  ProblemDraftSubtask,
  ProblemDraftTest,
} from "@/contracts/core/v1/schema";

export type ComponentRole = ProblemDraftComponent["role"];

const ROLE_FOLDERS: Record<ComponentRole, string> = {
  checker: "checkers",
  validator: "validators",
  interactor: "interactors",
  generator: "generators",
  solution: "solutions",
};

export const ROLE_LABELS: Record<ComponentRole, string> = {
  checker: "Чекер",
  validator: "Валидатор",
  interactor: "Интерактор",
  generator: "Генератор",
  solution: "Решение",
};

export const LIBRARY_FOLDER = "lib";
export const STATEMENT_FOLDER = "statements";
export const ASSET_FOLDER = "assets";

export type OpResult = {draft: ProblemDraft} | {error: string};

const newId = (): string => crypto.randomUUID();

/** Validates a file name or relative path the server accepts. */
export const isValidPath = (path: string): boolean =>
  path !== "" &&
  path.length <= 512 &&
  !path.startsWith("/") &&
  !path.endsWith("/") &&
  !path.includes("//") &&
  !path.includes("\\") &&
  !path.split("/").some((part) => part === "." || part === "..") &&
  path !== "problem.yaml" &&
  path !== ".gate" &&
  !path.startsWith(".gate/");

/** Points are exact hundredths; arithmetic runs on integers. */
export const toHundredths = (points: number): number => Math.round(points * 100);
const fromHundredths = (hundredths: number): number => hundredths / 100;
export const formatPoints = (points: number): string => points.toFixed(2).replace(/\.?0+$/, "");

export const sumPoints = (tests: ProblemDraftTest[]): number =>
  fromHundredths(tests.reduce((total, test) => total + toHundredths(test.points), 0));

export const addComponent = (
  draft: ProblemDraft,
  role: ComponentRole,
  fileName: string,
  language: string,
  source: string,
): OpResult => {
  const path = `${ROLE_FOLDERS[role]}/${fileName}`;
  if (!isValidPath(path) || fileName.includes("/")) {
    return {error: "Недопустимое имя файла"};
  }
  if (path in draft.files || draft.assets.some((asset) => asset.path === path)) {
    return {error: "Файл с таким именем уже существует"};
  }
  const component: ProblemDraftComponent = {id: newId(), role, path, language, dependencies: {}};
  const next: ProblemDraft = {
    ...draft,
    files: {...draft.files, [path]: source},
    components: [...draft.components, component],
  };
  // The first component of a singleton role becomes the selected one.
  if (role === "checker" && !next.checker_id) {
    next.checker_id = component.id;
  } else if (role === "validator" && !next.validator_id) {
    next.validator_id = component.id;
  } else if (role === "interactor" && !next.interactor_id) {
    next.interactor_id = component.id;
  } else if (role === "solution" && !next.main_solution_id) {
    next.main_solution_id = component.id;
  }
  return {draft: next};
};

export const removeComponent = (draft: ProblemDraft, id: string): OpResult => {
  const component = draft.components.find((c) => c.id === id);
  if (!component) {
    return {error: "Компонент не найден"};
  }
  const usedByTests = draft.tests.some(
    (test) => test.input.component_id === id || test.answer.component_id === id,
  );
  if (usedByTests) {
    return {error: "Компонент используется в тестах. Сначала измените эти тесты."};
  }
  const isDependency = (c: ProblemDraftComponent): boolean =>
    c.id !== id && Object.values(c.dependencies ?? {}).includes(component.path);
  if (draft.components.some(isDependency)) {
    return {error: "Файл используется как зависимость другого компонента"};
  }
  const files = {...draft.files};
  delete files[component.path];
  return {
    draft: {
      ...draft,
      files,
      components: draft.components.filter((c) => c.id !== id),
      checker_id: draft.checker_id === id ? null : draft.checker_id,
      validator_id: draft.validator_id === id ? null : draft.validator_id,
      interactor_id: draft.interactor_id === id ? null : draft.interactor_id,
      main_solution_id: draft.main_solution_id === id ? null : draft.main_solution_id,
    },
  };
};

export const setFile = (draft: ProblemDraft, path: string, content: string): ProblemDraft => ({
  ...draft,
  files: {...draft.files, [path]: content},
});

export const addLibraryFile = (draft: ProblemDraft, fileName: string, content: string): OpResult => {
  const path = `${LIBRARY_FOLDER}/${fileName}`;
  if (!isValidPath(path) || fileName.includes("/")) {
    return {error: "Недопустимое имя файла"};
  }
  if (path in draft.files) {
    return {error: "Файл с таким именем уже существует"};
  }
  return {draft: setFile(draft, path, content)};
};

export const removeLibraryFile = (draft: ProblemDraft, path: string): OpResult => {
  if (draft.components.some((c) => Object.values(c.dependencies ?? {}).includes(path))) {
    return {error: "Файл подключён как зависимость компонента"};
  }
  const files = {...draft.files};
  delete files[path];
  return {draft: {...draft, files}};
};

export const libraryFiles = (draft: ProblemDraft): string[] =>
  Object.keys(draft.files)
    .filter((path) => path.startsWith(`${LIBRARY_FOLDER}/`))
    .sort();

export const statementLanguages = (draft: ProblemDraft): string[] =>
  Object.keys(draft.files)
    .filter((path) => path.startsWith(`${STATEMENT_FOLDER}/`) && path.endsWith(".md"))
    .map((path) => path.slice(STATEMENT_FOLDER.length + 1, -".md".length))
    .sort();

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

export const removeSubtask = (draft: ProblemDraft, id: string): OpResult => {
  if (draft.tests.some((test) => test.subtask_id === id)) {
    return {error: "В подзадаче есть тесты. Сначала перенесите или удалите их."};
  }
  return {
    draft: {
      ...draft,
      subtasks: draft.subtasks
        .filter((s) => s.id !== id)
        .map((s) => ({...s, depends_on: s.depends_on.filter((dependency) => dependency !== id)})),
    },
  };
};

/** Reports whether making `subtask` depend on `dependency` would create a cycle. */
const wouldCreateCycle = (
  subtasks: ProblemDraftSubtask[],
  subtask: string,
  dependency: string,
): boolean => {
  const edges = new Map(subtasks.map((s) => [s.id, s.depends_on]));
  const seen = new Set<string>();
  const stack = [dependency];
  while (stack.length > 0) {
    const current = stack.pop() as string;
    if (current === subtask) {
      return true;
    }
    if (seen.has(current)) {
      continue;
    }
    seen.add(current);
    stack.push(...(edges.get(current) ?? []));
  }
  return false;
};

export const setSubtaskDependencies = (
  draft: ProblemDraft,
  id: string,
  dependencies: string[],
): OpResult => {
  if (dependencies.some((dependency) => dependency === id || wouldCreateCycle(draft.subtasks, id, dependency))) {
    return {error: "Зависимости подзадач не должны образовывать цикл"};
  }
  return {
    draft: {
      ...draft,
      subtasks: draft.subtasks.map((s) => (s.id === id ? {...s, depends_on: dependencies} : s)),
    },
  };
};

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

export const newGeneratedTest = (draft: ProblemDraft, subtaskId: string): ProblemDraftTest => ({
  id: newId(),
  ordinal: nextOrdinal(draft.tests),
  subtask_id: subtaskId,
  sample: false,
  points: 0,
  input: {
    mode: "generator",
    object_id: null,
    component_id: draft.components.find((c) => c.role === "generator")?.id ?? null,
    args: [],
    seed: 1,
  },
  answer: draft.type === "interactive"
    ? {mode: "interactive", object_id: null, component_id: null}
    : {mode: "generated", object_id: null, component_id: draft.main_solution_id},
});

export const newManualTest = (draft: ProblemDraft, subtaskId: string): ProblemDraftTest => ({
  id: newId(),
  ordinal: nextOrdinal(draft.tests),
  subtask_id: subtaskId,
  sample: false,
  points: 0,
  input: {mode: "manual", object_id: null, component_id: null, args: [], seed: null},
  answer: draft.type === "interactive"
    ? {mode: "interactive", object_id: null, component_id: null}
    : {mode: "provided", object_id: null, component_id: null},
});

/** Problems with every selected component the build requires, as reported to the author before building. */
export const buildProblems = (draft: ProblemDraft): string[] => {
  const problems: string[] = [];
  if (!draft.checker_id) {
    problems.push("Не выбран чекер");
  }
  if (draft.type === "interactive" && !draft.interactor_id) {
    problems.push("Для интерактивной задачи нужен интерактор");
  }
  if (draft.tests.length === 0) {
    problems.push("Нет ни одного теста");
  }
  if (draft.tests.some((t) => t.answer.mode === "generated" && !t.answer.component_id)) {
    problems.push("Для тестов с генерируемым ответом нужно главное решение");
  }
  if (draft.tests.some((t) => t.input.mode === "manual" && !t.input.object_id)) {
    problems.push("У некоторых ручных тестов не загружен вход");
  }
  if (draft.tests.some((t) => t.answer.mode === "provided" && !t.answer.object_id)) {
    problems.push("У некоторых тестов не загружен ответ");
  }
  if (draft.subtasks.length === 0) {
    problems.push("Нет ни одной подзадачи");
  }
  return problems;
};
