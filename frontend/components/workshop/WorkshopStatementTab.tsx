"use client";

import {ActionIcon, Anchor, Box, Button, FileInput, Group, Select, SimpleGrid, Stack, Table, Text, TextInput, Textarea, Tooltip} from "@mantine/core";
import {useClipboard} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {IconCopy, IconPencil, IconTrash, IconUpload} from "@tabler/icons-react";
import {useDeferredValue, useState} from "react";

import {ProblemStatement} from "@/components/problems/ProblemStatement";
import {SectionPaper} from "@/components/workshop/SectionPaper";

import {DraftGate} from "./draft/DraftGate";
import {emptyStatement, removeAsset, renameAsset, setStatement, STATEMENT_SECTIONS, type StatementSection} from "./draft/draftOps";
import {objectUrl, useSaveDraft, useUploadToDraft} from "./draft/useDraft";

import "../problems/Problem.css";

import type {ProblemDraft, ProblemDraftStatement} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

const SECTION_TITLES: Record<StatementSection, string> = {
  legend: "Условие",
  input: "Формат входных данных",
  output: "Формат выходных данных",
  interaction: "Протокол взаимодействия",
  notes: "Примечания",
  scoring: "Система оценки",
};

type StatementProps = {draft: ProblemDraft; problemId: string; statement: ProblemDraftStatement};

const Preview = ({draft, problemId, statement}: StatementProps): ReactNode => {
  const deferred = useDeferredValue(statement);
  // Sections refer to assets by name, read as participants' statements read them.
  const assetUrl = (name: string): string | undefined => {
    const asset = draft.assets.find((a) => a.name === name);
    return asset && objectUrl(problemId, asset.object_id);
  };
  return (
    <Stack gap="md" className="problem-statement">
      <Text fw={700} size="xl">
        {draft.title}
      </Text>
      {STATEMENT_SECTIONS.filter((key) => deferred[key].trim() !== "").map((key) => (
        <Box key={key}>
          {key !== "legend" && (
            <Text fw={600} mb={4}>
              {SECTION_TITLES[key]}
            </Text>
          )}
          <ProblemStatement value={deferred[key]} assetUrl={assetUrl} />
        </Box>
      ))}
    </Stack>
  );
};

const StatementForm = ({draft, problemId, statement: stored}: StatementProps): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const [statement, setLocal] = useState(stored);
  const changed = STATEMENT_SECTIONS.some((key) => statement[key] !== stored[key]);
  const sections = STATEMENT_SECTIONS.filter((key) => key !== "interaction" || draft.type === "interactive" || stored.interaction !== "");
  return (
    <Stack gap="md">
      <SimpleGrid cols={{base: 1, md: 2}} spacing="md">
        <Stack gap="sm">
          {sections.map((key) => (
            <Textarea
              key={key}
              label={SECTION_TITLES[key]}
              value={statement[key]}
              onChange={(e) => setLocal({...statement, [key]: e.currentTarget.value})}
              autosize
              minRows={key === "legend" ? 8 : 3}
              maxRows={30}
              styles={{input: {fontFamily: "var(--mantine-font-family-monospace)"}}}
            />
          ))}
        </Stack>
        <Box>
          <Text size="sm" fw={500} mb={4}>
            Предпросмотр
          </Text>
          <Preview draft={draft} problemId={problemId} statement={statement} />
        </Box>
      </SimpleGrid>
      <Group justify="flex-end">
        <Button
          disabled={!changed}
          loading={isSaving}
          onClick={() => void save((current) => setStatement(current, statement))}
        >
          Сохранить условие
        </Button>
      </Group>
    </Stack>
  );
};

const StatementEditor = ({draft, problemId}: {draft: ProblemDraft; problemId: string}): ReactNode => {
  const {save} = useSaveDraft(problemId);
  const languages = draft.statements.map((s) => s.language);
  const [language, setLanguage] = useState<string>(languages[0] ?? "ru");
  const [newLanguage, setNewLanguage] = useState("");
  const stored = draft.statements.find((s) => s.language === language) ?? emptyStatement(language);

  const addLanguage = async () => {
    const code = newLanguage.trim().toLowerCase();
    if (!/^[a-z]{2,8}(-[a-z0-9]{2,8})*$/.test(code)) {
      notifications.show({title: "Недопустимый код языка", message: "Например: ru или en", color: "red"});
      return;
    }
    if (!languages.includes(code) && !(await save((current) => setStatement(current, emptyStatement(code))))) {
      return;
    }
    setNewLanguage("");
    setLanguage(code);
  };

  const options = [...new Set([...languages, language])].map((value) => ({value, label: value.toUpperCase()}));

  return (
    <SectionPaper title="Условие">
      <Stack gap="md">
        <Group align="flex-end" wrap="wrap">
          <Select label="Язык" data={options} value={language} allowDeselect={false} onChange={(value) => value && setLanguage(value)} w={120} />
          <TextInput label="Новый язык" placeholder="en" value={newLanguage} onChange={(e) => setNewLanguage(e.currentTarget.value)} w={120} />
          <Button variant="light" disabled={newLanguage.trim() === ""} onClick={() => void addLanguage()}>
            Добавить
          </Button>
        </Group>
        <Text size="xs" c="dimmed">
          Название задачи берётся из черновика. Изображения подключаются по имени ресурса: ![](pic.png).
        </Text>
        <StatementForm key={language} draft={draft} problemId={problemId} statement={stored} />
      </Stack>
    </SectionPaper>
  );
};

const AssetsSection = ({draft, problemId}: {draft: ProblemDraft; problemId: string}): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const {upload, isUploading} = useUploadToDraft(problemId);
  const clipboard = useClipboard();
  const [file, setFile] = useState<File | null>(null);
  const [assetName, setAssetName] = useState("");

  const handleUpload = async () => {
    if (!file) {
      return;
    }
    try {
      await upload({change: {asset_name: assetName.trim() || file.name}, files: {asset: file}});
      setFile(null);
      setAssetName("");
    } catch {
      // The mutation already reported the failure.
    }
  };

  const rename = (name: string) => {
    const next = window.prompt("Новое имя ресурса", name)?.trim();
    if (next && next !== name) {
      void save((current) => renameAsset(current, name, next));
    }
  };

  return (
    <SectionPaper title="Ресурсы условия">
      <Stack gap="sm">
        <Group align="flex-end" wrap="wrap">
          <FileInput label="Файл" value={file} onChange={setFile} placeholder="Изображение, PDF…" style={{minWidth: 240}} />
          <TextInput
            label="Имя ресурса"
            placeholder={file?.name ?? "pic.png"}
            value={assetName}
            onChange={(e) => setAssetName(e.currentTarget.value)}
            style={{minWidth: 240}}
          />
          <Button
            leftSection={<IconUpload size={16} />}
            disabled={!file}
            loading={isUploading}
            onClick={() => void handleUpload()}
          >
            Загрузить
          </Button>
        </Group>
        {draft.assets.length === 0 ? (
          <Text size="sm" c="dimmed">
            Ресурсов нет.
          </Text>
        ) : (
          <Table withTableBorder>
            <Table.Tbody>
              {draft.assets.map((asset) => (
                <Table.Tr key={asset.name}>
                  <Table.Td>
                    <Anchor href={objectUrl(problemId, asset.object_id)} target="_blank" rel="noreferrer" size="sm">
                      {asset.name}
                    </Anchor>
                  </Table.Td>
                  <Table.Td w={120}>
                    <Group gap={4} wrap="nowrap">
                      <Tooltip label="Копировать Markdown-ссылку">
                        <ActionIcon variant="subtle" aria-label={`Копировать ссылку на ${asset.name}`} onClick={() => clipboard.copy(`![](${asset.name})`)}>
                          <IconCopy size={16} />
                        </ActionIcon>
                      </Tooltip>
                      <Tooltip label="Переименовать">
                        <ActionIcon variant="subtle" aria-label={`Переименовать ${asset.name}`} disabled={isSaving} onClick={() => rename(asset.name)}>
                          <IconPencil size={16} />
                        </ActionIcon>
                      </Tooltip>
                      <Tooltip label="Удалить">
                        <ActionIcon
                          variant="subtle"
                          color="red"
                          aria-label={`Удалить ${asset.name}`}
                          disabled={isSaving}
                          onClick={() => void save((current) => removeAsset(current, asset.name))}
                        >
                          <IconTrash size={16} />
                        </ActionIcon>
                      </Tooltip>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Stack>
    </SectionPaper>
  );
};

type Props = {
  problemId: string;
};

export const WorkshopStatementTab = ({problemId}: Props): ReactNode => (
  <Box p="lg">
    <Stack gap="lg" maw={1200} mx="auto">
      <DraftGate problemId={problemId}>
        {(draft) => (
          <>
            <StatementEditor draft={draft} problemId={problemId} />
            <AssetsSection draft={draft} problemId={problemId} />
          </>
        )}
      </DraftGate>
    </Stack>
  </Box>
);
