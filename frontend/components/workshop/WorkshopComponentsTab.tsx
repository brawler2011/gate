"use client";

import {Badge, Box, Button, Grid, Group, MultiSelect, NavLink, Select, Stack, Text, TextInput} from "@mantine/core";
import {useState} from "react";
import {useSearchParams} from "react-router-dom";

import {SectionPaper} from "@/components/workshop/SectionPaper";

import {DraftGate} from "./draft/DraftGate";
import {
  addProgram,
  isSingleRole,
  LANGUAGES,
  programsOf,
  removeProgram,
  ROLE_LABELS,
  setSingleProgram,
  updateProgram,
  type ComponentRole,
  type ListRole,
  type SingleRole,
} from "./draft/draftOps";
import {SourceEditor} from "./draft/SourceEditor";
import {componentTemplate} from "./draft/templates";
import {useSaveDraft} from "./draft/useDraft";

import type {ProblemDraft, ProblemDraftProgram} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

const LANGUAGE_OPTIONS = LANGUAGES.map((language) => ({value: language, label: language}));

type ProgramEditorProps = {
  draft: ProblemDraft;
  label: string;
  program: ProblemDraftProgram;
  saving: boolean;
  onChange: (patch: Partial<ProblemDraftProgram>) => void;
  onDelete: () => void;
  children?: ReactNode;
};

/** Language, library dependencies and source of one program. */
const ProgramEditor = (props: ProgramEditorProps): ReactNode => {
  const {draft, label, program, saving, onChange, onDelete, children} = props;
  return (
    <SectionPaper>
      <Stack gap="md">
        <Group align="flex-end" wrap="wrap">
          <Select
            label="Язык"
            data={LANGUAGE_OPTIONS}
            value={program.language}
            allowDeselect={false}
            onChange={(value) => value && onChange({language: value})}
            w={200}
          />
          {children}
        </Group>
        <MultiSelect
          label="Подключаемые файлы"
          description="Файлы из библиотеки, доступные программе при сборке под своим именем"
          data={draft.library.map((file) => ({value: file.id, label: file.name}))}
          value={program.dependencies}
          onChange={(dependencies) => onChange({dependencies})}
          searchable
          placeholder={draft.library.length === 0 ? "Библиотека пуста" : "Не подключено"}
        />
        <SourceEditor
          key={draft.revision}
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
  const [language, setLanguage] = useState("cpp");
  return (
    <>
      {withName && (
        <TextInput label="Новая программа" placeholder="main" value={name} onChange={(e) => setName(e.currentTarget.value)} />
      )}
      <Select
        label="Язык"
        data={LANGUAGE_OPTIONS}
        value={language}
        allowDeselect={false}
        onChange={(value) => setLanguage(value ?? "cpp")}
      />
      <Button
        variant="light"
        disabled={withName && name.trim() === ""}
        loading={saving}
        onClick={() => void onCreate(name.trim(), language).then((ok) => ok && setName(""))}
      >
        Создать
      </Button>
    </>
  );
};

type PanelProps<Role> = {draft: ProblemDraft; problemId: string; role: Role};

const SinglePanel = ({draft, problemId, role}: PanelProps<SingleRole>): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const program = draft[role];
  if (!program) {
    return (
      <SectionPaper title={ROLE_LABELS[role]}>
        <Stack gap="xs" maw={320}>
          <Text size="sm" c="dimmed">
            {ROLE_LABELS[role]} ещё не создан.
          </Text>
          <NewProgramForm
            withName={false}
            saving={isSaving}
            onCreate={(_, language) =>
              save((current) =>
                setSingleProgram(current, role, {
                  language,
                  source: componentTemplate(role, language),
                  dependencies: [],
                }),
              )
            }
          />
        </Stack>
      </SectionPaper>
    );
  }
  return (
    <ProgramEditor
      draft={draft}
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
      const next = addProgram(current, role, newName, language, componentTemplate(role, language));
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
                description={program.language}
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
            draft={draft}
            label={selected.name}
            program={selected}
            saving={isSaving}
            onChange={(patch) => void save((current) => updateProgram(current, role, selected.id, patch))}
            onDelete={() => void save((current) => removeProgram(current, role, selected.id))}
          >
            <RenameControl
              key={`${selected.id}:${draft.revision}`}
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
