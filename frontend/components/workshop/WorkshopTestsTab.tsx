"use client";

import {ActionIcon, Anchor, Badge, Box, Button, Group, MultiSelect, NumberInput, Select, Stack, Table, Text, TextInput, Tooltip} from "@mantine/core";
import {IconArrowDown, IconArrowUp, IconPencil, IconTrash} from "@tabler/icons-react";
import {useState} from "react";

import {SectionPaper} from "@/components/workshop/SectionPaper";

import {DraftGate} from "./draft/DraftGate";
import {
  addSubtask,
  formatPoints,
  moveTest,
  newGeneratedTest,
  newManualTest,
  removeSubtask,
  removeTest,
  setSubtaskDependencies,
  subtaskLabel,
  sumPoints,
  upsertTest,
} from "./draft/draftOps";
import {objectUrl, useSaveDraft, useUploadToDraft} from "./draft/useDraft";
import {TestEditorModal, type TestFiles} from "./tests/TestEditorModal";

import type {ProblemDraft, ProblemDraftSubtask, ProblemDraftTest} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

type PanelProps = {
  draft: ProblemDraft;
  problemId: string;
};

const SubtasksSection = ({draft, problemId}: PanelProps): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const [name, setName] = useState("");

  const update = (id: string, patch: Partial<ProblemDraftSubtask>) =>
    save((current) => ({...current, subtasks: current.subtasks.map((s) => (s.id === id ? {...s, ...patch} : s))}));

  return (
    <SectionPaper title="Подзадачи">
      <Stack gap="sm">
        <Text size="xs" c="dimmed">
          «Все тесты» начисляет сумму весов группы только если пройдена вся группа; «Каждый тест» суммирует веса
          пройденных тестов. Подзадача начисляет баллы, только если пройдены все подзадачи, от которых она зависит.
        </Text>
        {draft.subtasks.length > 0 && (
          <Table withTableBorder>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Название</Table.Th>
                <Table.Th>Начисление</Table.Th>
                <Table.Th>Видно на контесте</Table.Th>
                <Table.Th>Зависит от</Table.Th>
                <Table.Th>Баллы</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {draft.subtasks.map((subtask) => (
                <Table.Tr key={subtask.id}>
                  <Table.Td>
                    <TextInput
                      size="xs"
                      defaultValue={subtask.name}
                      placeholder={`${subtaskLabel(subtask)}, без группы`}
                      onBlur={(e) => {
                        const name = e.currentTarget.value.trim();
                        if (name !== subtask.name) {
                          void update(subtask.id, {name});
                        }
                      }}
                    />
                  </Table.Td>
                  <Table.Td>
                    <Select
                      size="xs"
                      data={[
                        {value: "complete", label: "Все тесты"},
                        {value: "each", label: "Каждый тест"},
                      ]}
                      value={subtask.policy}
                      allowDeselect={false}
                      onChange={(value) => value && void update(subtask.id, {policy: value as ProblemDraftSubtask["policy"]})}
                    />
                  </Table.Td>
                  <Table.Td>
                    <Select
                      size="xs"
                      data={[
                        {value: "complete", label: "Все тесты"},
                        {value: "icpc", label: "До первой ошибки"},
                        {value: "points", label: "Только баллы"},
                        {value: "none", label: "Ничего"},
                      ]}
                      value={subtask.feedback}
                      allowDeselect={false}
                      onChange={(value) => value && void update(subtask.id, {feedback: value as ProblemDraftSubtask["feedback"]})}
                    />
                  </Table.Td>
                  <Table.Td>
                    <MultiSelect
                      size="xs"
                      data={draft.subtasks
                        .filter((s) => s.id !== subtask.id)
                        .map((s) => ({value: s.id, label: subtaskLabel(s)}))}
                      value={subtask.depends_on}
                      placeholder="—"
                      onChange={(ids) => void save((current) => setSubtaskDependencies(current, subtask.id, ids))}
                    />
                  </Table.Td>
                  <Table.Td>
                    <Text size="sm">{formatPoints(subtask.points)}</Text>
                  </Table.Td>
                  <Table.Td>
                    <Tooltip label="Удалить подзадачу">
                      <ActionIcon variant="subtle" color="red" onClick={() => void save((current) => removeSubtask(current, subtask.id))}>
                        <IconTrash size={16} />
                      </ActionIcon>
                    </Tooltip>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
        <Group align="flex-end">
          <TextInput label="Новая подзадача" placeholder="Подзадача 1" value={name} onChange={(e) => setName(e.currentTarget.value)} />
          <Button
            variant="light"
            disabled={name.trim() === ""}
            loading={isSaving}
            onClick={async () => {
              if (await save((current) => addSubtask(current, name.trim()))) {
                setName("");
              }
            }}
          >
            Добавить
          </Button>
        </Group>
      </Stack>
    </SectionPaper>
  );
};

const describeInput = (test: ProblemDraftTest, draft: ProblemDraft, problemId: string): ReactNode => {
  if (test.input.mode === "manual") {
    return test.input.object_id ? (
      <Anchor size="sm" href={objectUrl(problemId, test.input.object_id)} target="_blank" rel="noreferrer">
        файл
      </Anchor>
    ) : (
      <Badge color="orange" variant="light">
        нет файла
      </Badge>
    );
  }
  const generator = draft.generators.find((g) => g.name === test.input.generator);
  return (
    <Text size="sm" ff="monospace">
      {generator?.name ?? "?"} {test.input.args.join(" ")}    </Text>
  );
};

const describeAnswer = (test: ProblemDraftTest, problemId: string): ReactNode => {
  if (test.answer.mode === "interactive") {
    return <Text size="sm" c="dimmed">интерактор</Text>;
  }
  if (test.answer.mode === "generated") {
    return <Text size="sm">решение</Text>;
  }
  return test.answer.object_id ? (
    <Anchor size="sm" href={objectUrl(problemId, test.answer.object_id)} target="_blank" rel="noreferrer">
      файл
    </Anchor>
  ) : (
    <Badge color="orange" variant="light">
      нет файла
    </Badge>
  );
};

const TestsSection = ({draft, problemId}: PanelProps): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const {upload, isUploading} = useUploadToDraft(problemId);
  const [editing, setEditing] = useState<{test: ProblemDraftTest; isNew: boolean} | null>(null);
  const subtaskNames = new Map(draft.subtasks.map((s) => [s.id, subtaskLabel(s)]));

  const create = (kind: "manual" | "generator") => {
    const subtask = draft.subtasks.at(-1);
    if (subtask) {
      const test = kind === "manual" ? newManualTest(draft, subtask.id) : newGeneratedTest(draft, subtask.id);
      setEditing({test, isNew: true});
    }
  };

  const submit = async (test: ProblemDraftTest, files: TestFiles) => {
    const ok =
      files.input || files.answer
        ? await upload({
          change: {test},
          files: {input: files.input ?? undefined, answer: files.answer ?? undefined},
        }).then(() => true, () => false)
        : await save((current) => upsertTest(current, test));
    if (ok) {
      setEditing(null);
    }
  };

  return (
    <SectionPaper title="Тесты">
      <Stack gap="sm">
        <Group justify="space-between">
          <Text size="sm" c="dimmed">
            Сумма весов: {formatPoints(sumPoints(draft.tests))}
            {draft.max_score !== null && ` · максимальный балл задачи: ${formatPoints(draft.max_score)}`}
          </Text>
          <Group gap="xs">
            <Button variant="light" disabled={draft.subtasks.length === 0} onClick={() => create("manual")}>
              Тест из файлов
            </Button>
            <Button variant="light" disabled={draft.subtasks.length === 0} onClick={() => create("generator")}>
              Сгенерированный тест
            </Button>
          </Group>
        </Group>
        {draft.subtasks.length === 0 && (
          <Text size="sm" c="dimmed">
            Сначала добавьте подзадачу: каждый тест принадлежит ровно одной подзадаче.
          </Text>
        )}
        {draft.tests.length > 0 && (
          <Table withTableBorder highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th w={50}>№</Table.Th>
                <Table.Th>Подзадача</Table.Th>
                <Table.Th>Вход</Table.Th>
                <Table.Th>Ответ</Table.Th>
                <Table.Th>Вес</Table.Th>
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {draft.tests.map((test, index) => (
                <Table.Tr key={test.id}>
                  <Table.Td>
                    {test.ordinal}
                    {test.sample && (
                      <Badge ml={4} size="xs" variant="light">
                        пример
                      </Badge>
                    )}
                    {test.pretest && (
                      <Badge ml={4} size="xs" variant="light" color="grape">
                        претест
                      </Badge>
                    )}
                  </Table.Td>
                  <Table.Td>{subtaskNames.get(test.subtask_id) ?? "?"}</Table.Td>
                  <Table.Td>{describeInput(test, draft, problemId)}</Table.Td>
                  <Table.Td>{describeAnswer(test, problemId)}</Table.Td>
                  <Table.Td>
                    <NumberInput
                      size="xs"
                      w={90}
                      defaultValue={test.points}
                      min={0}
                      decimalScale={2}
                      hideControls
                      onBlur={(e) => {
                        const value = Number(e.currentTarget.value.replace(",", "."));
                        if (Number.isFinite(value) && Math.round(value * 100) !== Math.round(test.points * 100)) {
                          void save((current) => upsertTest(current, {...test, points: Math.round(value * 100) / 100}));
                        }
                      }}
                    />
                  </Table.Td>
                  <Table.Td>
                    <Group gap={2} wrap="nowrap">
                      <ActionIcon variant="subtle" disabled={index === 0 || isSaving} onClick={() => void save((current) => moveTest(current, test.id, -1))}>
                        <IconArrowUp size={16} />
                      </ActionIcon>
                      <ActionIcon variant="subtle" disabled={index === draft.tests.length - 1 || isSaving} onClick={() => void save((current) => moveTest(current, test.id, 1))}>
                        <IconArrowDown size={16} />
                      </ActionIcon>
                      <ActionIcon variant="subtle" onClick={() => setEditing({test, isNew: false})}>
                        <IconPencil size={16} />
                      </ActionIcon>
                      <ActionIcon variant="subtle" color="red" onClick={() => void save((current) => removeTest(current, test.id))}>
                        <IconTrash size={16} />
                      </ActionIcon>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Stack>
      {editing && (
        <TestEditorModal
          key={editing.test.id}
          draft={draft}
          test={editing.test}
          isNew={editing.isNew}
          saving={isSaving || isUploading}
          onClose={() => setEditing(null)}
          onSubmit={(test, files) => void submit(test, files)}
        />
      )}
    </SectionPaper>
  );
};

type Props = {
  problemId: string;
};

export const WorkshopTestsTab = ({problemId}: Props): ReactNode => (
  <Box p="lg">
    <Stack gap="lg" maw={1100} mx="auto">
      <DraftGate problemId={problemId}>
        {(draft) => (
          <>
            <SubtasksSection draft={draft} problemId={problemId} />
            <TestsSection draft={draft} problemId={problemId} />
          </>
        )}
      </DraftGate>
    </Stack>
  </Box>
);
