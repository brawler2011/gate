"use client";

import {Badge, Box, Button, Code, Grid, Group, Menu, Modal, NavLink, ScrollArea, Stack, Text, TextInput} from "@mantine/core";
import {useState} from "react";
import {useSearchParams} from "react-router-dom";

import {SectionPaper} from "@/components/workshop/SectionPaper";
import {useSystemLibrary} from "@/lib/systemLibrary";

import {DraftGate} from "./draft/DraftGate";
import {
  addDraftFile,
  addLibraryFile,
  addSystemFile,
  removeDraftFile,
  removeLibraryFile,
  startAutoUpdate,
  stopAutoUpdate,
  updateDraftFile,
  updateLibraryFile,
} from "./draft/draftOps";
import {SourceEditor} from "./draft/SourceEditor";
import {useSaveDraft} from "./draft/useDraft";

import type {FileList} from "./draft/draftOps";
import type {ProblemDraft, ProblemDraftLibraryFile} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

type RenameProps = {name: string; description?: string; saving: boolean; onRename: (name: string) => void};

const RenameFile = ({name, description = "Под этим именем программы жюри подключают файл", saving, onRename}: RenameProps): ReactNode => {
  const [value, setValue] = useState(name);
  return (
    <Grid gutter="xs" align="flex-end">
      <Grid.Col span="auto">
        <TextInput label="Имя файла" description={description} value={value} onChange={(e) => setValue(e.currentTarget.value)} />
      </Grid.Col>
      <Grid.Col span="content">
        <Button variant="default" disabled={value.trim() === "" || value.trim() === name} loading={saving} onClick={() => onRename(value.trim())}>
          Переименовать
        </Button>
      </Grid.Col>
    </Grid>
  );
};

const SolutionName = ({draft, problemId}: {draft: ProblemDraft; problemId: string}): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const [value, setValue] = useState(draft.solution_name);
  return (
    <Stack gap="xs">
      <TextInput
        label="Имя решения"
        description="Под этим именем грейдер на Python или Java импортирует решение (import aplusb, класс aplusb); точка входа — grader.py или grader.java. Без имени эти языки недоступны."
        placeholder="aplusb"
        value={value}
        onChange={(e) => setValue(e.currentTarget.value)}
      />
      <Button
        variant="default"
        disabled={value.trim() === draft.solution_name}
        loading={isSaving}
        onClick={() => void save((current) => ({...current, solution_name: value.trim()}))}
      >
        Сохранить имя
      </Button>
    </Stack>
  );
};

type SystemFileProps = {
  file: ProblemDraftLibraryFile;
  saving: boolean;
  onStop: (text: string) => void;
  onDelete: () => void;
};

const SystemFile = ({file, saving, onStop, onDelete}: SystemFileProps): ReactNode => {
  const system = useSystemLibrary()?.find((s) => s.name === file.name);
  return (
    <Stack gap="md">
      <Group gap="xs">
        <Text ff="monospace" size="sm" fw={600}>
          {file.name}
        </Text>
        <Badge variant="light">Автообновление</Badge>
      </Group>
      <Text size="sm" c="dimmed">
        Системный файл {system?.version ? `версии ${system.version}` : ""}: каждая сборка берёт его текущую версию. Чтобы изменить файл, отключите
        автообновление — в черновике останется редактируемая копия текущего текста.
      </Text>
      <ScrollArea.Autosize mah={480}>
        <Code block>{system?.content ?? ""}</Code>
      </ScrollArea.Autosize>
      <Group justify="space-between">
        <Button variant="subtle" color="red" loading={saving} onClick={onDelete}>
          Удалить
        </Button>
        <Button variant="default" disabled={!system} loading={saving} onClick={() => system && onStop(system.content)}>
          Отключить автообновление
        </Button>
      </Group>
    </Stack>
  );
};

type StartProps = {name: string; version: string; saving: boolean; onStart: () => void};

const StartAutoUpdate = ({name, version, saving, onStart}: StartProps): ReactNode => {
  const [confirming, setConfirming] = useState(false);
  return (
    <>
      <Button variant="default" loading={saving} onClick={() => setConfirming(true)}>
        Включить автообновление
      </Button>
      <Modal opened={confirming} onClose={() => setConfirming(false)} title="Включить автообновление?" centered>
        <Stack>
          <Text size="sm">
            Текст {name} заменится системным {version}, и каждая сборка будет брать его текущую версию.
            Тесты генераторов могут измениться.
          </Text>
          <Group justify="flex-end">
            <Button variant="default" onClick={() => setConfirming(false)}>
              Отмена
            </Button>
            <Button
              onClick={() => {
                setConfirming(false);
                onStart();
              }}
            >
              Заменить
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
};

const LibraryPanel = ({draft, problemId}: {draft: ProblemDraft; problemId: string}): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const [params, setParams] = useSearchParams();
  const [newName, setNewName] = useState("");
  const selected = draft.library.find((file) => file.name === params.get("file")) ?? draft.library[0];

  const open = (name: string) => {
    const next = new URLSearchParams(params);
    next.set("file", name);
    setParams(next, {replace: true});
  };

  const handleCreate = async () => {
    const name = newName.trim();
    if (await save((current) => addLibraryFile(current, name))) {
      setNewName("");
      open(name);
    }
  };

  const handleRename = async (from: string, to: string) => {
    if (await save((current) => updateLibraryFile(current, from, {name: to}))) {
      open(to);
    }
  };

  const system = useSystemLibrary() ?? [];
  const missing = system.filter((s) => !draft.library.some((file) => file.name === s.name));
  const systemCopy = system.find((s) => s.name === selected?.name);

  const handleAddSystem = async (name: string) => {
    if (await save((current) => addSystemFile(current, name))) {
      open(name);
    }
  };

  return (
    <Grid gutter="lg">
      <Grid.Col span={{base: 12, md: 4}}>
        <SectionPaper title="Библиотека">
          <Stack gap="xs">
            {draft.library.length === 0 && (
              <Text size="sm" c="dimmed">
                Файлов пока нет. Файлы библиотеки доступны чекеру, валидатору, интерактору и генераторам
                под своими именами.
              </Text>
            )}
            {draft.library.map((file) => (
              <NavLink
                key={file.name}
                active={file.name === selected?.name}
                label={file.name}
                description={file.auto_update ? "автообновление" : undefined}
                onClick={() => open(file.name)}
              />
            ))}
            {missing.length > 0 && (
              <Menu position="bottom-start">
                <Menu.Target>
                  <Button variant="default" loading={isSaving}>
                    Добавить системный файл
                  </Button>
                </Menu.Target>
                <Menu.Dropdown>
                  {missing.map((s) => (
                    <Menu.Item key={s.name} onClick={() => void handleAddSystem(s.name)}>
                      {s.name} {s.version}
                    </Menu.Item>
                  ))}
                </Menu.Dropdown>
              </Menu>
            )}
            <TextInput label="Новый файл" placeholder="helper.h" value={newName} onChange={(e) => setNewName(e.currentTarget.value)} />
            <Button variant="light" disabled={newName.trim() === ""} loading={isSaving} onClick={() => void handleCreate()}>
              Создать
            </Button>
          </Stack>
        </SectionPaper>
      </Grid.Col>
      <Grid.Col span={{base: 12, md: 8}}>
        {selected?.auto_update && (
          <SectionPaper>
            <SystemFile
              key={selected.name}
              file={selected}
              saving={isSaving}
              onStop={(text) => void save((current) => stopAutoUpdate(current, selected.name, text))}
              onDelete={() => void save((current) => removeLibraryFile(current, selected.name))}
            />
          </SectionPaper>
        )}
        {selected && !selected.auto_update && (
          <SectionPaper>
            <Stack gap="md">
              {systemCopy && (
                <StartAutoUpdate
                  name={selected.name}
                  version={systemCopy.version}
                  saving={isSaving}
                  onStart={() => void save((current) => startAutoUpdate(current, selected.name))}
                />
              )}
              <RenameFile
                key={`rename:${selected.name}`}
                name={selected.name}
                saving={isSaving}
                onRename={(name) => void handleRename(selected.name, name)}
              />
              <SourceEditor
                key={`source:${selected.name}`}
                label={selected.name}
                content={selected.content}
                saving={isSaving}
                onSave={(content) => void save((current) => updateLibraryFile(current, selected.name, {content}))}
                onDelete={() => void save((current) => removeLibraryFile(current, selected.name))}
              />
            </Stack>
          </SectionPaper>
        )}
      </Grid.Col>
    </Grid>
  );
};

const FILE_LISTS: Record<FileList, {title: string; empty: string; placeholder: string; rename: string}> = {
  graders: {
    title: "Грейдеры",
    empty: "Грейдеров нет: решение — отдельная программа. Грейдеры компилируются вместе с каждым решением, "
      + "и отправить можно только на языках, исходник грейдера которых есть среди файлов.",
    placeholder: "grader.cpp",
    rename: "Под этим именем решение подключает файл",
  },
  attachments: {
    title: "Файлы для участников",
    empty: "Файлов нет. Участники скачивают их вместе с условием, например упрощённый грейдер и заголовок.",
    placeholder: "grader.cpp",
    rename: "Под этим именем участники получат файл",
  },
};

const FilesPanel = ({draft, problemId, list}: {draft: ProblemDraft; problemId: string; list: FileList}): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const [params, setParams] = useSearchParams();
  const [newName, setNewName] = useState("");
  const files = draft[list];
  const selected = files.find((file) => file.name === params.get(list)) ?? files[0];
  const text = FILE_LISTS[list];

  const open = (name: string) => {
    const next = new URLSearchParams(params);
    next.set(list, name);
    setParams(next, {replace: true});
  };

  const handleCreate = async () => {
    const name = newName.trim();
    if (await save((current) => addDraftFile(current, list, name))) {
      setNewName("");
      open(name);
    }
  };

  const handleRename = async (from: string, to: string) => {
    if (await save((current) => updateDraftFile(current, list, from, {name: to}))) {
      open(to);
    }
  };

  return (
    <Grid gutter="lg">
      <Grid.Col span={{base: 12, md: 4}}>
        <SectionPaper title={text.title}>
          <Stack gap="xs">
            {files.length === 0 && (
              <Text size="sm" c="dimmed">
                {text.empty}
              </Text>
            )}
            {files.map((file) => (
              <NavLink
                key={file.name}
                active={file.name === selected?.name}
                label={file.name}
                onClick={() => open(file.name)}
              />
            ))}
            <TextInput label="Новый файл" placeholder={text.placeholder} value={newName} onChange={(e) => setNewName(e.currentTarget.value)} />
            <Button variant="light" disabled={newName.trim() === ""} loading={isSaving} onClick={() => void handleCreate()}>
              Создать
            </Button>
            {list === "graders" && <SolutionName key={draft.solution_name} draft={draft} problemId={problemId} />}
          </Stack>
        </SectionPaper>
      </Grid.Col>
      <Grid.Col span={{base: 12, md: 8}}>
        {selected && (
          <SectionPaper>
            <Stack gap="md">
              <RenameFile
                key={`rename:${selected.name}`}
                name={selected.name}
                description={text.rename}
                saving={isSaving}
                onRename={(name) => void handleRename(selected.name, name)}
              />
              <SourceEditor
                key={`source:${selected.name}`}
                label={selected.name}
                content={selected.content}
                saving={isSaving}
                onSave={(content) => void save((current) => updateDraftFile(current, list, selected.name, {content}))}
                onDelete={() => void save((current) => removeDraftFile(current, list, selected.name))}
              />
            </Stack>
          </SectionPaper>
        )}
      </Grid.Col>
    </Grid>
  );
};

type Props = {
  problemId: string;
};

export const WorkshopLibTab = ({problemId}: Props): ReactNode => (
  <Box p="lg">
    <Stack gap="lg" maw={1200} mx="auto">
      <DraftGate problemId={problemId}>
        {(draft) => (
          <>
            <LibraryPanel draft={draft} problemId={problemId} />
            <FilesPanel draft={draft} problemId={problemId} list="graders" />
            <FilesPanel draft={draft} problemId={problemId} list="attachments" />
          </>
        )}
      </DraftGate>
    </Stack>
  </Box>
);
