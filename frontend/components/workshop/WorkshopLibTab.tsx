"use client";

import {Box, Button, Grid, NavLink, Stack, Text, TextInput} from "@mantine/core";
import {useState} from "react";
import {useSearchParams} from "react-router-dom";

import {SectionPaper} from "@/components/workshop/SectionPaper";

import {DraftGate} from "./draft/DraftGate";
import {addLibraryFile, libraryFiles, LIBRARY_FOLDER, removeLibraryFile, setFile} from "./draft/draftOps";
import {SourceEditor} from "./draft/SourceEditor";
import {useSaveDraft} from "./draft/useDraft";

import type {ProblemDraft} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

const LibraryPanel = ({draft, problemId}: {draft: ProblemDraft; problemId: string}): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const [params, setParams] = useSearchParams();
  const [newName, setNewName] = useState("");
  const files = libraryFiles(draft);
  const selected = files.find((path) => path === params.get("file")) ?? files[0];

  const open = (path: string) => {
    const next = new URLSearchParams(params);
    next.set("file", path);
    setParams(next, {replace: true});
  };

  const handleCreate = async () => {
    const name = newName.trim();
    if (await save((current) => addLibraryFile(current, name, ""))) {
      setNewName("");
      open(`${LIBRARY_FOLDER}/${name}`);
    }
  };

  return (
    <Grid gutter="lg">
      <Grid.Col span={{base: 12, md: 4}}>
        <SectionPaper title="Библиотека">
          <Stack gap="xs">
            {files.length === 0 && (
              <Text size="sm" c="dimmed">
                Файлов пока нет. Например, добавьте testlib.h и подключите его к компонентам.
              </Text>
            )}
            {files.map((path) => (
              <NavLink
                key={path}
                active={path === selected}
                label={path.slice(LIBRARY_FOLDER.length + 1)}
                onClick={() => open(path)}
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
            <SourceEditor
              key={`${selected}:${draft.revision}`}
              path={selected}
              content={draft.files[selected] ?? ""}
              saving={isSaving}
              onSave={(content) => void save((current) => setFile(current, selected, content))}
              onDelete={() => void save((current) => removeLibraryFile(current, selected))}
            />
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
