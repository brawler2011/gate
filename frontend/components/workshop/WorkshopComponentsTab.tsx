"use client";

import {Badge, Box, Button, Code, Grid, Group, NavLink, ScrollArea, Select, Stack, Text, TextInput} from "@mantine/core";
import {useState} from "react";
import {useSearchParams} from "react-router-dom";

import {SectionPaper} from "@/components/workshop/SectionPaper";
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
  ROLE_LABELS,
  setSingleProgram,
  standardChecker,
  updateProgram,
  type ComponentRole,
  type ListRole,
  type SingleRole,
} from "./draft/draftOps";
import {SourceEditor} from "./draft/SourceEditor";
import {componentTemplate} from "./draft/templates";
import {useSaveDraft} from "./draft/useDraft";

import type {ProblemDraft, ProblemDraftChecker, ProblemDraftProgram} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

/** The catalog configurations as select options. */
const languageOptions = (catalog: SupportedLanguage[]) =>
  catalog.map((language) => ({value: language.id, label: language.name}));

/** A starter source in the configuration `id` of the catalog. */
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

/** Language and source of one program. */
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

/** The C++ configurations of the catalog, which standard checkers compile in. */
const cppLanguages = (catalog: SupportedLanguage[]) => catalog.filter((language) => language.extensions.includes("cpp"));

/** A select of the standard checkers by name and description. */
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

/** A standard checker: its current source, read-only until the author makes a copy. */
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
  const selected = programs.find((p) => p.id === params.get("component")) ?? programs[0];

  const open = (id: string) => {
    const next = new URLSearchParams(params);
    next.set("component", id);
    setParams(next, {replace: true});
  };

  const create = async (newName: string, language: string): Promise<boolean> => {
    let createdId: string | undefined;
    const ok = await save((current) => {
      const next = addProgram(current, role, newName, language, template(catalog, role, language));
      createdId = programsOf(next, role).at(-1)?.id;
      return next;
    });
    if (ok && createdId) {
      setParams(new URLSearchParams({component: createdId}), {replace: true});
    }
    return ok;
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
                key={program.id}
                active={program.id === selected?.id}
                label={program.name}
                description={languageName(catalog, program.language)}
                rightSection={
                  role === "solution" && draft.main_solution_id === program.id ? (
                    <Badge size="xs" variant="light">
                      Главное
                    </Badge>
                  ) : undefined
                }
                onClick={() => open(program.id)}
              />
            ))}
            <NewProgramForm withName saving={isSaving} onCreate={create} />
          </Stack>
        </SectionPaper>
      </Grid.Col>
      <Grid.Col span={{base: 12, md: 8}}>
        {selected ? (
          <ProgramEditor
            key={selected.id}
            label={selected.name}
            program={selected}
            saving={isSaving}
            onChange={(patch) => void save((current) => updateProgram(current, role, selected.id, patch))}
            onDelete={() => void save((current) => removeProgram(current, role, selected.id))}
          >
            <RenameControl
              key={`${selected.id}:${selected.name}`}
              name={selected.name}
              saving={isSaving}
              onRename={(name) => void save((current) => updateProgram(current, role, selected.id, {name}))}
            />
            {role === "solution" && draft.main_solution_id !== selected.id && (
              <Button
                variant="light"
                loading={isSaving}
                onClick={() => void save((current) => ({...current, main_solution_id: selected.id}))}
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
