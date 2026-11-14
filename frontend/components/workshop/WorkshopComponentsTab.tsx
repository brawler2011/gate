"use client";

import {Badge, Box, Button, Grid, Group, MultiSelect, NavLink, Select, Stack, Text, TextInput} from "@mantine/core";
import {useState} from "react";
import {useSearchParams} from "react-router-dom";

import {SectionPaper} from "@/components/workshop/SectionPaper";

import {DraftGate} from "./draft/DraftGate";
import {
  addComponent,
  LANGUAGE_EXTENSIONS,
  languageForPath,
  LIBRARY_FOLDER,
  libraryFiles,
  removeComponent,
  ROLE_LABELS,
  setComponentLanguage,
  setFile,
  SINGLETON_ROLES,
  type ComponentRole,
} from "./draft/draftOps";
import {SourceEditor} from "./draft/SourceEditor";
import {componentTemplate} from "./draft/templates";
import {useSaveDraft} from "./draft/useDraft";

import type {ProblemDraft, ProblemDraftComponent} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

const libraryName = (path: string): string => path.slice(LIBRARY_FOLDER.length + 1);

const LANGUAGE_OPTIONS = Object.entries(LANGUAGE_EXTENSIONS).map(([language, extension]) => ({
  value: language,
  label: `${language} (${extension})`,
}));

type PanelProps = {
  draft: ProblemDraft;
  problemId: string;
  role: ComponentRole;
};

const ComponentsPanel = ({draft, problemId, role}: PanelProps): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const [params, setParams] = useSearchParams();
  const [newName, setNewName] = useState("");
  const [newLanguage, setNewLanguage] = useState<string>("cpp");

  const components = draft.components.filter((c) => c.role === role);
  const selected = components.find((c) => c.id === params.get("component")) ?? components[0];
  const canCreate = !SINGLETON_ROLES.has(role) || components.length === 0;
  const libs = libraryFiles(draft);

  const open = (component: ProblemDraftComponent) => {
    const next = new URLSearchParams(params);
    next.set("component", component.id);
    setParams(next, {replace: true});
  };

  const handleCreate = async () => {
    const name = newName.trim();
    let createdId: string | undefined;
    const ok = await save((current) => {
      const result = addComponent(current, role, name, newLanguage, componentTemplate(role, newLanguage));
      if ("draft" in result) {
        createdId = result.draft.components.at(-1)?.id;
      }
      return result;
    });
    if (ok) {
      setNewName("");
      if (createdId) {
        setParams(new URLSearchParams({component: createdId}), {replace: true});
      }
    }
  };

  const updateComponent = (id: string, patch: Partial<ProblemDraftComponent>) =>
    save((current) => ({...current, components: current.components.map((c) => (c.id === id ? {...c, ...patch} : c))}));

  return (
    <Grid gutter="lg">
      <Grid.Col span={{base: 12, md: 4}}>
        <SectionPaper title={ROLE_LABELS[role]}>
          <Stack gap="xs">
            {components.length === 0 && (
              <Text size="sm" c="dimmed">
                Файлов пока нет.
              </Text>
            )}
            {components.map((component) => (
              <NavLink
                key={component.id}
                active={component.id === selected?.id}
                label={component.path.split("/").pop()}
                description={languageForPath(component.path)}
                rightSection={
                  role === "solution" && draft.main_solution_id === component.id ? (
                    <Badge size="xs" variant="light">
                      Главное
                    </Badge>
                  ) : undefined
                }
                onClick={() => open(component)}
              />
            ))}
            {canCreate ? (
              <>
                <TextInput
                  label="Новый файл"
                  description="Имя без расширения; расширение задаёт язык"
                  placeholder="main"
                  value={newName}
                  onChange={(e) => setNewName(e.currentTarget.value)}
                />
                <Select
                  label="Язык"
                  data={LANGUAGE_OPTIONS}
                  value={newLanguage}
                  allowDeselect={false}
                  onChange={(value) => setNewLanguage(value ?? "cpp")}
                />
                <Button variant="light" disabled={newName.trim() === ""} loading={isSaving} onClick={() => void handleCreate()}>
                  Создать
                </Button>
              </>
            ) : (
              <Text size="xs" c="dimmed">
                У задачи может быть только один файл этой роли.
              </Text>
            )}
          </Stack>
        </SectionPaper>
      </Grid.Col>
      <Grid.Col span={{base: 12, md: 8}}>
        {selected ? (
          <SectionPaper>
            <Stack gap="md">
              <Group align="flex-end" wrap="wrap">
                <Select
                  label="Язык"
                  description="Меняет расширение файла"
                  data={LANGUAGE_OPTIONS}
                  value={languageForPath(selected.path) || null}
                  allowDeselect={false}
                  onChange={(value) =>
                    value && void save((current) => setComponentLanguage(current, selected.id, value))
                  }
                  w={200}
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
              </Group>
              <MultiSelect
                label="Подключаемые файлы"
                description="Файлы из библиотеки, доступные компоненту при сборке под своим именем"
                data={libs.map((path) => ({value: path, label: libraryName(path)}))}
                value={Object.values(selected.dependencies ?? {})}
                onChange={(paths) =>
                  void updateComponent(selected.id, {
                    dependencies: Object.fromEntries(paths.map((path) => [libraryName(path), path])),
                  })
                }
                searchable
                placeholder={libs.length === 0 ? "Библиотека пуста" : "Не подключено"}
              />
              <SourceEditor
                key={`${selected.id}:${draft.revision}`}
                path={selected.path}
                content={draft.files[selected.path] ?? ""}
                saving={isSaving}
                onSave={(content) => void save((current) => setFile(current, selected.path, content))}
                onDelete={() => void save((current) => removeComponent(current, selected.id))}
              />
            </Stack>
          </SectionPaper>
        ) : (
          <SectionPaper>
            <Text size="sm" c="dimmed">
              Создайте файл, чтобы начать.
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
        {(draft) => <ComponentsPanel draft={draft} problemId={problemId} role={role} />}
      </DraftGate>
    </Stack>
  </Box>
);
