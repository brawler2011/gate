"use client";

import {ActionIcon, Anchor, Box, Button, FileInput, Group, Select, SimpleGrid, Stack, Table, Text, TextInput, Textarea, Tooltip} from "@mantine/core";
import {useClipboard} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {IconCopy, IconTrash, IconUpload} from "@tabler/icons-react";
import {useDeferredValue, useState} from "react";
import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

import "katex/dist/katex.min.css";

import {SectionPaper} from "@/components/workshop/SectionPaper";

import {DraftGate} from "./draft/DraftGate";
import {isValidPath, setFile, statementLanguages, STATEMENT_FOLDER} from "./draft/draftOps";
import {objectUrl, useSaveDraft, useUploadToDraft} from "./draft/useDraft";

import "../problems/Problem.css";

import type {ProblemDraft} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

const SECTION_MARKERS = /<!--\s*(title|legend|input|output|notes|interaction|scoring)\s*-->/gi;

const SECTION_TITLES: Record<string, string> = {
  legend: "Условие",
  input: "Формат входных данных",
  output: "Формат выходных данных",
  interaction: "Протокол взаимодействия",
  notes: "Примечания",
  scoring: "Система оценки",
};

type Section = {key: string; body: string};

/** Splits a statement the same way the server does: text before the first marker is the legend. */
const splitSections = (markdown: string): Section[] => {
  const markers = [...markdown.matchAll(SECTION_MARKERS)];
  if (markers.length === 0) {
    return [{key: "legend", body: markdown.trim()}];
  }
  const sections: Section[] = [{key: "legend", body: markdown.slice(0, markers[0].index).trim()}];
  markers.forEach((marker, index) => {
    const end = markers[index + 1]?.index ?? markdown.length;
    const start = (marker.index ?? 0) + marker[0].length;
    sections.push({key: marker[1].toLowerCase(), body: markdown.slice(start, end).trim()});
  });
  return sections.filter((section) => section.body !== "" && section.key !== "title");
};

const Preview = ({title, markdown}: {title: string; markdown: string}): ReactNode => {
  const deferred = useDeferredValue(markdown);
  return (
    <Stack gap="md" className="problem-statement">
      <Text fw={700} size="xl">
        {title}
      </Text>
      {splitSections(deferred).map((section, index) => (
        <Box key={`${section.key}-${index}`}>
          {section.key !== "legend" || index > 0 ? (
            <Text fw={600} mb={4}>
              {SECTION_TITLES[section.key] ?? section.key}
            </Text>
          ) : null}
          <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
            {section.body}
          </ReactMarkdown>
        </Box>
      ))}
    </Stack>
  );
};

const StatementEditor = ({draft, problemId}: {draft: ProblemDraft; problemId: string}): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const languages = statementLanguages(draft);
  const [language, setLanguage] = useState<string>(languages[0] ?? "ru");
  const [newLanguage, setNewLanguage] = useState("");
  const path = `${STATEMENT_FOLDER}/${language}.md`;
  const stored = draft.files[path] ?? "";
  const [text, setText] = useState(stored);

  const selectLanguage = (value: string | null) => {
    if (value) {
      setLanguage(value);
      setText(draft.files[`${STATEMENT_FOLDER}/${value}.md`] ?? "");
    }
  };

  const addLanguage = async () => {
    const code = newLanguage.trim().toLowerCase();
    if (!/^[a-z]{2,8}(-[a-z0-9]{2,8})*$/.test(code)) {
      notifications.show({title: "Недопустимый код языка", message: "Например: ru или en", color: "red"});
      return;
    }
    if (languages.includes(code)) {
      selectLanguage(code);
      return;
    }
    if (await save((current) => setFile(current, `${STATEMENT_FOLDER}/${code}.md`, ""))) {
      setNewLanguage("");
      selectLanguage(code);
    }
  };

  const options = [...new Set([...languages, language])].map((value) => ({value, label: value.toUpperCase()}));

  return (
    <SectionPaper title="Условие">
      <Stack gap="md">
        <Group align="flex-end" wrap="wrap">
          <Select label="Язык" data={options} value={language} allowDeselect={false} onChange={selectLanguage} w={120} />
          <TextInput label="Новый язык" placeholder="en" value={newLanguage} onChange={(e) => setNewLanguage(e.currentTarget.value)} w={120} />
          <Button variant="light" disabled={newLanguage.trim() === ""} onClick={() => void addLanguage()}>
            Добавить
          </Button>
        </Group>
        <Text size="xs" c="dimmed">
          Название задачи берётся из черновика. Разделы отделяются маркерами: {"<!-- input -->"}, {"<!-- output -->"},{" "}
          {"<!-- notes -->"}, {"<!-- interaction -->"}, {"<!-- scoring -->"}; текст до первого маркера — условие.
        </Text>
        <SimpleGrid cols={{base: 1, md: 2}} spacing="md">
          <Textarea
            label="Markdown"
            value={text}
            onChange={(e) => setText(e.currentTarget.value)}
            autosize
            minRows={16}
            maxRows={40}
            styles={{input: {fontFamily: "var(--mantine-font-family-monospace)"}}}
          />
          <Box>
            <Text size="sm" fw={500} mb={4}>
              Предпросмотр
            </Text>
            <Preview title={draft.title} markdown={text} />
          </Box>
        </SimpleGrid>
        <Group justify="flex-end">
          <Button
            disabled={text === stored}
            loading={isSaving}
            onClick={() => void save((current) => setFile(current, path, text))}
          >
            Сохранить условие
          </Button>
        </Group>
      </Stack>
    </SectionPaper>
  );
};

const AssetsSection = ({draft, problemId}: {draft: ProblemDraft; problemId: string}): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const {upload, isUploading} = useUploadToDraft(problemId);
  const clipboard = useClipboard();
  const [file, setFile] = useState<File | null>(null);
  const [assetPath, setAssetPath] = useState("");

  const handleUpload = async () => {
    const target = assetPath.trim() || file?.name || "";
    if (!file || !isValidPath(target) || target.startsWith(`${STATEMENT_FOLDER}/`)) {
      notifications.show({title: "Недопустимый путь ресурса", message: "Укажите относительный путь, например images/pic.png", color: "red"});
      return;
    }
    try {
      await upload({part: "asset", assetPath: target, file});
      setFile(null);
      setAssetPath("");
    } catch {
      // The mutation already reported the failure.
    }
  };

  return (
    <SectionPaper title="Ресурсы условия">
      <Stack gap="sm">
        <Group align="flex-end" wrap="wrap">
          <FileInput label="Файл" value={file} onChange={setFile} placeholder="Изображение, PDF…" style={{minWidth: 240}} />
          <TextInput
            label="Путь в условии"
            placeholder={file?.name ?? "images/pic.png"}
            value={assetPath}
            onChange={(e) => setAssetPath(e.currentTarget.value)}
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
                <Table.Tr key={asset.path}>
                  <Table.Td>
                    <Anchor href={objectUrl(problemId, asset.object_id)} target="_blank" rel="noreferrer" size="sm">
                      {asset.path}
                    </Anchor>
                  </Table.Td>
                  <Table.Td w={90}>
                    <Group gap={4} wrap="nowrap">
                      <Tooltip label="Копировать Markdown-ссылку">
                        <ActionIcon variant="subtle" onClick={() => clipboard.copy(`![](${asset.path})`)}>
                          <IconCopy size={16} />
                        </ActionIcon>
                      </Tooltip>
                      <Tooltip label="Удалить">
                        <ActionIcon
                          variant="subtle"
                          color="red"
                          disabled={isSaving}
                          onClick={() =>
                            void save((current) => ({
                              ...current,
                              assets: current.assets.filter((a) => a.path !== asset.path),
                            }))
                          }
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
