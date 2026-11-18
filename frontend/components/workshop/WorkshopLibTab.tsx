"use client";

import {Badge, Box, Button, Code, Grid, Group, Menu, NavLink, ScrollArea, Stack, Text, TextInput} from "@mantine/core";
import {useState} from "react";
import {useSearchParams} from "react-router-dom";

import {SectionPaper} from "@/components/workshop/SectionPaper";
import {useSystemLibrary} from "@/lib/systemLibrary";

import {DraftGate} from "./draft/DraftGate";
import {addLibraryFile, addSystemFile, removeLibraryFile, stopAutoUpdate, updateLibraryFile} from "./draft/draftOps";
import {SourceEditor} from "./draft/SourceEditor";
import {useSaveDraft} from "./draft/useDraft";

import type {ProblemDraft, ProblemDraftLibraryFile} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

type RenameProps = {name: string; saving: boolean; onRename: (name: string) => void};

const RenameFile = ({name, saving, onRename}: RenameProps): ReactNode => {
  const [value, setValue] = useState(name);
  return (
    <Grid gutter="xs" align="flex-end">
      <Grid.Col span="auto">
        <TextInput label="Имя файла" description="Под этим именем программы подключают файл" value={value} onChange={(e) => setValue(e.currentTarget.value)} />
      </Grid.Col>
      <Grid.Col span="content">
        <Button variant="default" disabled={value.trim() === "" || value.trim() === name} loading={saving} onClick={() => onRename(value.trim())}>
          Переименовать
        </Button>
      </Grid.Col>
    </Grid>
  );
};

type SystemFileProps = {
  file: ProblemDraftLibraryFile;
  saving: boolean;
  onStop: (text: string) => void;
  onDelete: () => void;
};

/** An auto-updated file: the system text, read-only until auto-update is turned off. */
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

const LibraryPanel = ({draft, problemId}: {draft: ProblemDraft; problemId: string}): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const [params, setParams] = useSearchParams();
  const [newName, setNewName] = useState("");
  const selected = draft.library.find((file) => file.id === params.get("file")) ?? draft.library[0];

  const open = (id: string) => {
    const next = new URLSearchParams(params);
    next.set("file", id);
    setParams(next, {replace: true});
  };

  const handleCreate = async () => {
    let createdId = "";
    const ok = await save((current) => {
      const result = addLibraryFile(current, newName.trim());
      createdId = result.id;
      return result.draft;
    });
    if (ok) {
      setNewName("");
      open(createdId);
    }
  };

  const system = useSystemLibrary() ?? [];
  const missing = system.filter((s) => !draft.library.some((file) => file.auto_update && file.name === s.name));

  const handleAddSystem = async (name: string) => {
    let createdId = "";
    const ok = await save((current) => {
      const result = addSystemFile(current, name);
      createdId = result.id;
      return result.draft;
    });
    if (ok) {
      open(createdId);
    }
  };

  return (
    <Grid gutter="lg">
      <Grid.Col span={{base: 12, md: 4}}>
        <SectionPaper title="Библиотека">
          <Stack gap="xs">
            {draft.library.length === 0 && (
              <Text size="sm" c="dimmed">
                Файлов пока нет.
              </Text>
            )}
            {draft.library.map((file) => (
              <NavLink
                key={file.id}
                active={file.id === selected?.id}
                label={file.name}
                description={file.auto_update ? "автообновление" : undefined}
                onClick={() => open(file.id)}
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
              key={selected.id}
              file={selected}
              saving={isSaving}
              onStop={(text) => void save((current) => stopAutoUpdate(current, selected.id, text))}
              onDelete={() => void save((current) => removeLibraryFile(current, selected.id))}
            />
          </SectionPaper>
        )}
        {selected && !selected.auto_update && (
          <SectionPaper>
            <Stack gap="md">
              <RenameFile
                key={`${selected.id}:${selected.name}`}
                name={selected.name}
                saving={isSaving}
                onRename={(name) => void save((current) => updateLibraryFile(current, selected.id, {name}))}
              />
              <SourceEditor
                key={selected.id}
                label={selected.name}
                content={selected.content}
                saving={isSaving}
                onSave={(content) => void save((current) => updateLibraryFile(current, selected.id, {content}))}
                onDelete={() => void save((current) => removeLibraryFile(current, selected.id))}
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
      <DraftGate problemId={problemId}>{(draft) => <LibraryPanel draft={draft} problemId={problemId} />}</DraftGate>
    </Stack>
  </Box>
);
