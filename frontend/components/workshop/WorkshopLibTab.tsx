"use client";

import {Box, Button, Grid, NavLink, Stack, Text, TextInput} from "@mantine/core";
import {useState} from "react";
import {useSearchParams} from "react-router-dom";

import {SectionPaper} from "@/components/workshop/SectionPaper";

import {DraftGate} from "./draft/DraftGate";
import {addLibraryFile, removeLibraryFile, updateLibraryFile} from "./draft/draftOps";
import {SourceEditor} from "./draft/SourceEditor";
import {useSaveDraft} from "./draft/useDraft";

import type {ProblemDraft} from "@/contracts/core/v1/schema";
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

  return (
    <Grid gutter="lg">
      <Grid.Col span={{base: 12, md: 4}}>
        <SectionPaper title="Библиотека">
          <Stack gap="xs">
            {draft.library.length === 0 && (
              <Text size="sm" c="dimmed">
                Файлов пока нет. Например, добавьте testlib.h и подключите его к программам.
              </Text>
            )}
            {draft.library.map((file) => (
              <NavLink
                key={file.id}
                active={file.id === selected?.id}
                label={file.name}
                onClick={() => open(file.id)}
              />
            ))}
            <TextInput label="Новый файл" placeholder="testlib.h" value={newName} onChange={(e) => setNewName(e.currentTarget.value)} />
            <Button variant="light" disabled={newName.trim() === ""} loading={isSaving} onClick={() => void handleCreate()}>
              Создать
            </Button>
          </Stack>
        </SectionPaper>
      </Grid.Col>
      <Grid.Col span={{base: 12, md: 8}}>
        {selected ? (
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
        ) : null}
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
