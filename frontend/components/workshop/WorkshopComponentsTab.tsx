"use client";

import {Badge, Box, Button, Code, Grid, Group, MultiSelect, NavLink, NumberInput, ScrollArea, Select, Stack, Text, TextInput} from "@mantine/core";
import {useState} from "react";
import {useSearchParams} from "react-router-dom";

import {SectionPaper} from "@/components/workshop/SectionPaper";
import {useProblemBuilds} from "@/components/workshop/WorkshopPackagesTab";
import {languageName, useLanguages, type SupportedLanguage} from "@/lib/languages";
import {useStandardCheckers, useSystemLibrary} from "@/lib/systemLibrary";

import {DraftGate} from "./draft/DraftGate";
import {
  addProgram,
  customChecker,
  customizeChecker,
  isSingleRole,
  programsOf,
  removeProgram,
  renameProgram,
  ROLE_LABELS,
  setSingleProgram,
  setSolutionExpectation,
  setValidatorSubtasks,
  standardChecker,
  subtaskLabel,
  updateProgram,
  type ComponentRole,
  type ListRole,
  type SingleRole,
} from "./draft/draftOps";
import {SourceEditor} from "./draft/SourceEditor";
import {componentTemplate} from "./draft/templates";
import {useSaveDraft} from "./draft/useDraft";

import type {
  ProblemBuildSolution,
  ProblemDraft,
  ProblemDraftChecker,
  ProblemDraftProgram,
  ProblemSolutionVerdict,
} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

const languageOptions = (catalog: SupportedLanguage[]) =>
  catalog.map((language) => ({value: language.id, label: language.name}));

const template = (catalog: SupportedLanguage[], role: ComponentRole, id: string): string =>
  componentTemplate(role, catalog.find((language) => language.id === id));

type ProgramEditorProps = {
  label: string;
  program: ProblemDraftProgram;
  saving: boolean;
  onChange: (patch: Partial<ProblemDraftProgram>) => void;
  onDelete: () => void;
  children?: ReactNode;
};

const ProgramEditor = (props: ProgramEditorProps): ReactNode => {
  const {label, program, saving, onChange, onDelete, children} = props;
  const catalog = useLanguages();
  return (
    <SectionPaper>
      <Stack gap="md">
        <Group align="flex-end" wrap="wrap">
          <Select
            label="Язык"
            description="Компилятор или интерпретатор и его версия"
            data={languageOptions(catalog)}
            value={program.language}
            allowDeselect={false}
            onChange={(value) => value && onChange({language: value})}
            w={260}
          />
          {children}
        </Group>
        <SourceEditor
          label={label}
          content={program.source}
          saving={saving}
          onSave={(source) => onChange({source})}
          onDelete={onDelete}
        />
      </Stack>
    </SectionPaper>
  );
};

const NewProgramForm = ({withName, saving, onCreate}: {
  withName: boolean;
  saving: boolean;
  onCreate: (name: string, language: string) => Promise<boolean>;
}): ReactNode => {
  const [name, setName] = useState("");
  const catalog = useLanguages();
  const [chosen, setLanguage] = useState<string | null>(null);
  const language = chosen ?? catalog[0]?.id ?? null;
  return (
    <>
      {withName && (
        <TextInput label="Новая программа" placeholder="main" value={name} onChange={(e) => setName(e.currentTarget.value)} />
      )}
      <Select
        label="Язык"
        data={languageOptions(catalog)}
        value={language}
        allowDeselect={false}
        onChange={setLanguage}
      />
      <Button
        variant="light"
        disabled={(withName && name.trim() === "") || !language}
        loading={saving}
        onClick={() => language && void onCreate(name.trim(), language).then((ok) => ok && setName(""))}
      >
        Создать
      </Button>
    </>
  );
};

const cppLanguages = (catalog: SupportedLanguage[]) => catalog.filter((language) => language.extensions.includes("cpp"));

type StandardCheckerSelectProps = {value: string | null; onChange: (name: string) => void};

const StandardCheckerSelect = ({value, onChange}: StandardCheckerSelectProps): ReactNode => {
  const checkers = useStandardCheckers() ?? [];
  return (
    <Select
      label="Стандартный чекер"
      description="Готовый чекер testlib"
      placeholder="Выберите чекер"
      data={checkers.map((checker) => ({value: checker.name, label: `${checker.name} — ${checker.description}`}))}
      value={value}
      searchable
      allowDeselect={false}
      onChange={(name) => name && onChange(name)}
      w={520}
    />
  );
};

type NewStandardCheckerProps = {saving: boolean; onChoose: (name: string, language: string) => Promise<boolean>};

const NewStandardChecker = ({saving, onChoose}: NewStandardCheckerProps): ReactNode => {
  const [name, setName] = useState<string | null>(null);
  const language = cppLanguages(useLanguages())[0]?.id;
  return (
    <>
      <StandardCheckerSelect value={name} onChange={setName} />
      <Button variant="light" disabled={!name || !language} loading={saving} onClick={() => name && language && void onChoose(name, language)}>
        Выбрать
      </Button>
    </>
  );
};

type StandardCheckerProps = {
  checker: ProblemDraftChecker;
  saving: boolean;
  onChange: (patch: Partial<ProblemDraftChecker>) => void;
  onCustomize: (source: string) => void;
  onDelete: () => void;
};

const StandardChecker = ({checker, saving, onChange, onCustomize, onDelete}: StandardCheckerProps): ReactNode => {
  const catalog = useLanguages();
  const standard = useStandardCheckers()?.find((s) => s.name === checker.standard);
  return (
    <SectionPaper>
      <Stack gap="md">
        <Group align="flex-end" wrap="wrap">
          <StandardCheckerSelect value={checker.standard} onChange={(name) => onChange({standard: name})} />
          <Select
            label="Язык"
            description="Компилятор C++"
            data={languageOptions(cppLanguages(catalog))}
            value={checker.language}
            allowDeselect={false}
            onChange={(value) => value && onChange({language: value})}
            w={260}
          />
        </Group>
        <Text size="sm" c="dimmed">
          Каждая сборка берёт текущую версию чекера и компилирует его с системным testlib.h. Чтобы изменить чекер,
          сделайте редактируемую копию.
        </Text>
        <ScrollArea.Autosize mah={480}>
          <Code block>{standard?.content ?? ""}</Code>
        </ScrollArea.Autosize>
        <Group justify="space-between">
          <Button variant="subtle" color="red" loading={saving} onClick={onDelete}>
            Удалить
          </Button>
          <Button variant="default" disabled={!standard} loading={saving} onClick={() => standard && onCustomize(standard.content)}>
            Сделать редактируемую копию
          </Button>
        </Group>
      </Stack>
    </SectionPaper>
  );
};

type PanelProps<Role> = {draft: ProblemDraft; problemId: string; role: Role};

const VERDICTS: {value: ProblemSolutionVerdict; label: string}[] = [
  {value: "OK", label: "OK — принято"},
  {value: "WA", label: "WA — неправильный ответ"},
  {value: "PE", label: "PE — ошибка представления"},
  {value: "TLE", label: "TLE — превышено время"},
  {value: "MLE", label: "MLE — превышена память"},
  {value: "OLE", label: "OLE — превышен вывод"},
  {value: "RE", label: "RE — ошибка выполнения"},
  {value: "FAIL", label: "FAIL — сбой чекера"},
  {value: "POINTS", label: "POINTS — частичные баллы"},
  {value: "CE", label: "CE — ошибка компиляции"},
];

// Results of the newest build that got as far as judging the solutions.
type SolutionResults = {revision: number; solutions: ProblemBuildSolution[]};

const useLastSolutionResults = (problemId: string, enabled: boolean): SolutionResults | null => {
  const builds = useProblemBuilds(problemId, enabled);
  const build = builds.data?.find((item) => item.solutions);
  return build?.solutions ? {revision: build.revision, solutions: build.solutions} : null;
};

const ResultBadge = ({result}: {result: ProblemBuildSolution}): ReactNode => (
  <Badge size="xs" variant="light" color={result.mismatch ? "red" : "green"} title={result.mismatch || undefined}>
    {result.verdict}
  </Badge>
);

const describeResult = (result: ProblemBuildSolution): string =>
  [
    result.test === null ? result.verdict : `${result.verdict} на тесте ${result.test}`,
    result.verdict === "CE" ? null : `${result.time_ms} мс`,
    result.points === null ? null : `${result.points} баллов`,
  ]
    .filter(Boolean)
    .join(", ");

type ExpectationProps = {
  draft: ProblemDraft;
  name: string;
  result: ProblemBuildSolution | undefined;
  revision: number | undefined;
  onChange: (change: (current: ProblemDraft) => ProblemDraft) => void;
};

const SolutionExpectation = ({draft, name, result, revision, onChange}: ExpectationProps): ReactNode => {
  const solution = draft.solutions.find((s) => s.name === name);
  if (!solution) {
    return null;
  }
  return (
    <Stack gap={4}>
      <Group align="flex-end" wrap="wrap">
        <MultiSelect
          label="Ожидаемые вердикты"
          description="На каждом тесте OK или один из них; пусто — не запускать при сборке"
          data={VERDICTS}
          value={solution.verdicts}
          onChange={(verdicts) =>
            onChange((current) =>
              setSolutionExpectation(current, name, {verdicts: verdicts as ProblemSolutionVerdict[]}),
            )
          }
          clearable
          w={420}
        />
        {draft.type === "scoring" && (
          <NumberInput
            label="Ожидаемые баллы"
            description="Пусто — любые"
            key={name}
            defaultValue={solution.points ?? ""}
            min={0}
            decimalScale={2}
            onBlur={(e) => {
              const text = e.currentTarget.value.replace(",", ".").trim();
              const points = text === "" ? null : Math.round(Number(text) * 100) / 100;
              if ((points === null || Number.isFinite(points)) && points !== solution.points) {
                onChange((current) => setSolutionExpectation(current, name, {points}));
              }
            }}
            w={160}
          />
        )}
      </Group>
      {result && (
        <Text size="sm" c={result.mismatch ? "red" : "dimmed"}>
          Сборка ревизии {revision}: {describeResult(result)}
          {result.mismatch && ` — не совпадает с ожидаемым (${result.mismatch})`}
        </Text>
      )}
    </Stack>
  );
};

const SinglePanel = ({draft, problemId, role}: PanelProps<SingleRole>): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const catalog = useLanguages();
  const systemFiles = useSystemLibrary()?.map((file) => file.name) ?? [];
  const program = draft[role];
  if (!program) {
    return (
      <SectionPaper title={ROLE_LABELS[role]}>
        <Stack gap="xs" maw={role === "checker" ? 520 : 320}>
          <Text size="sm" c="dimmed">
            {ROLE_LABELS[role]} ещё не создан.
          </Text>
          {role === "checker" && (
            <NewStandardChecker
              saving={isSaving}
              onChoose={(name, language) => save((current) => setSingleProgram(current, "checker", standardChecker(name, language)))}
            />
          )}
          <NewProgramForm
            withName={false}
            saving={isSaving}
            onCreate={(_, language) => {
              const source = template(catalog, role, language);
              return save((current) =>
                role === "checker"
                  ? setSingleProgram(current, "checker", customChecker(language, source))
                  : setSingleProgram(current, "interactor", {language, source}),
              );
            }}
          />
        </Stack>
      </SectionPaper>
    );
  }
  const checker = role === "checker" ? draft.checker : null;
  if (checker?.standard) {
    return (
      <StandardChecker
        checker={checker}
        saving={isSaving}
        onChange={(patch) => void save((current) => setSingleProgram(current, "checker", {...(current.checker ?? checker), ...patch}))}
        onCustomize={(source) => void save((current) => customizeChecker(current, source, systemFiles))}
        onDelete={() => void save((current) => setSingleProgram(current, "checker", null))}
      />
    );
  }
  return (
    <ProgramEditor
      label={ROLE_LABELS[role]}
      program={program}
      saving={isSaving}
      onChange={(patch) =>
        void save((current) => setSingleProgram(current, role, {...(current[role] ?? program), ...patch}))
      }
      onDelete={() => void save((current) => setSingleProgram(current, role, null))}
    />
  );
};

const RenameControl = ({name, saving, onRename}: {
  name: string;
  saving: boolean;
  onRename: (name: string) => void;
}): ReactNode => {
  const [value, setValue] = useState(name);
  return (
    <>
      <TextInput label="Название" value={value} onChange={(e) => setValue(e.currentTarget.value)} w={200} />
      <Button
        variant="default"
        disabled={value.trim() === "" || value.trim() === name}
        loading={saving}
        onClick={() => onRename(value.trim())}
      >
        Переименовать
      </Button>
    </>
  );
};

const ListPanel = ({draft, problemId, role}: PanelProps<ListRole>): ReactNode => {
  const catalog = useLanguages();
  const {save, isSaving} = useSaveDraft(problemId);
  const [params, setParams] = useSearchParams();
  const programs = programsOf(draft, role);
  const results = useLastSolutionResults(problemId, role === "solution");
  const resultOf = (name: string) => results?.solutions.find((r) => r.name === name);
  // Keep editor identity across renames to preserve unsaved source.
  const [renamed, setRenamed] = useState<Record<string, string>>({});
  const editorKey = (name: string): string => renamed[name] ?? `name:${name}`;
  // While a rename is saved, the URL may still hold the old name.
  const component = params.get("component");
  const selected =
    programs.find((p) => p.name === component) ??
    programs.find((p) => component !== null && editorKey(p.name) === editorKey(component)) ??
    programs[0];

  const open = (name: string) => {
    const next = new URLSearchParams(params);
    next.set("component", name);
    setParams(next, {replace: true});
  };

  const create = async (newName: string, language: string): Promise<boolean> => {
    const ok = await save((current) => addProgram(current, role, newName, language, template(catalog, role, language)));
    if (ok) {
      // Never the identity of a program renamed away from this name.
      setRenamed((current) => ({...current, [newName]: crypto.randomUUID()}));
      open(newName);
    }
    return ok;
  };

  const rename = async (from: string, to: string): Promise<void> => {
    // The saved draft renders before save resolves, so the identity moves first.
    const previous = renamed;
    const identity = editorKey(from);
    setRenamed({...renamed, [to]: identity});
    if (await save((current) => renameProgram(current, role, from, to))) {
      // `from` keeps resolving to the renamed program until the URL follows.
      open(to);
    } else {
      setRenamed(previous);
    }
  };

  return (
    <Grid gutter="lg">
      <Grid.Col span={{base: 12, md: 4}}>
        <SectionPaper title={ROLE_LABELS[role]}>
          <Stack gap="xs">
            {programs.length === 0 && (
              <Text size="sm" c="dimmed">
                Программ пока нет.
              </Text>
            )}
            {programs.map((program) => (
              <NavLink
                key={program.name}
                active={program.name === selected?.name}
                label={program.name}
                description={languageName(catalog, program.language)}
                rightSection={
                  role === "solution" ? (
                    <Group gap={4} wrap="nowrap">
                      {draft.main_solution === program.name && (
                        <Badge size="xs" variant="light">
                          Главное
                        </Badge>
                      )}
                      {resultOf(program.name) && (
                        <ResultBadge result={resultOf(program.name) as ProblemBuildSolution} />
                      )}
                    </Group>
                  ) : undefined
                }
                onClick={() => open(program.name)}
              />
            ))}
            <NewProgramForm withName saving={isSaving} onCreate={create} />
          </Stack>
        </SectionPaper>
      </Grid.Col>
      <Grid.Col span={{base: 12, md: 8}}>
        {selected ? (
          <ProgramEditor
            key={editorKey(selected.name)}
            label={selected.name}
            program={selected}
            saving={isSaving}
            onChange={(patch) => void save((current) => updateProgram(current, role, selected.name, patch))}
            onDelete={() => void save((current) => removeProgram(current, role, selected.name))}
          >
            <RenameControl
              name={selected.name}
              saving={isSaving}
              onRename={(name) => void rename(selected.name, name)}
            />
            {role === "validator" && (
              <MultiSelect
                label="Подзадачи"
                description="Пусто — проверяет все тесты"
                data={draft.subtasks.map((s) => ({value: s.id, label: subtaskLabel(s)}))}
                value={draft.validators.find((v) => v.name === selected.name)?.subtasks ?? []}
                onChange={(subtasks) => void save((current) => setValidatorSubtasks(current, selected.name, subtasks))}
                clearable
                w={320}
              />
            )}
            {role === "solution" && (
              <SolutionExpectation
                draft={draft}
                name={selected.name}
                result={resultOf(selected.name)}
                revision={results?.revision}
                onChange={(change) => void save(change)}
              />
            )}
            {role === "solution" && draft.main_solution !== selected.name && (
              <Button
                variant="light"
                loading={isSaving}
                onClick={() => void save((current) => ({...current, main_solution: selected.name}))}
              >
                Сделать главным
              </Button>
            )}
          </ProgramEditor>
        ) : (
          <SectionPaper>
            <Text size="sm" c="dimmed">
              Создайте программу, чтобы начать.
            </Text>
          </SectionPaper>
        )}
      </Grid.Col>
    </Grid>
  );
};

type Props = {
  problemId: string;
  role: ComponentRole;
};

export const WorkshopComponentsTab = ({problemId, role}: Props): ReactNode => (
  <Box p="lg">
    <Stack gap="lg" maw={1200} mx="auto">
      <DraftGate problemId={problemId}>
        {(draft) =>
          isSingleRole(role) ? (
            <SinglePanel draft={draft} problemId={problemId} role={role} />
          ) : (
            <ListPanel draft={draft} problemId={problemId} role={role} />
          )
        }
      </DraftGate>
    </Stack>
  </Box>
);
