"use client";

import {ActionIcon, Anchor, Badge, Box, Button, Checkbox, Group, MultiSelect, NumberInput, Select, Stack, Table, Text, TextInput, Tooltip} from "@mantine/core";
import {IconArrowDown, IconArrowUp, IconPencil, IconTrash} from "@tabler/icons-react";
import {useState} from "react";

import {SectionPaper} from "@/components/workshop/SectionPaper";

import {DraftGate} from "./draft/DraftGate";
import {
  addGroup,
  formatPoints,
  groupLabel,
  groupMaxPoints,
  groupRows,
  isInteractive,
  moveTest,
  newGeneratedTest,
  newManualTest,
  removeGroup,
  removeTest,
  requirableGroups,
  setGroupTestPoints,
  sumPoints,
  updateGroup,
  upsertTest,
} from "./draft/draftOps";
import {objectUrl, useSaveDraft, useUploadToDraft} from "./draft/useDraft";
import {TestEditorModal, type TestFiles} from "./tests/TestEditorModal";

import type {ProblemDraft, ProblemDraftGroup, ProblemDraftTest} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

type PanelProps = {
  draft: ProblemDraft;
  problemId: string;
};

const AGGREGATES = [
  {value: "min", label: "Всё или ничего (минимум)"},
  {value: "sum", label: "Сумма"},
  {value: "max", label: "Максимум"},
  {value: "avg", label: "Среднее"},
];

const FEEDBACK = [
  {value: "complete", label: "Все тесты"},
  {value: "icpc", label: "До первой ошибки"},
  {value: "points", label: "Только баллы"},
  {value: "none", label: "Ничего"},
];

// Arguments are passed as argv without shell expansion.
const parseArgs = (text: string): string[] => text.split(/\s+/).filter((part) => part !== "");

type ArgsInputProps = {label: string; value: string[]; onSave: (args: string[]) => void};

const ArgsInput = ({label, value, onSave}: ArgsInputProps): ReactNode => (
  <TextInput
    size="xs"
    label={label}
    defaultValue={value.join(" ")}
    onBlur={(e) => {
      const args = parseArgs(e.currentTarget.value);
      if (args.join(" ") !== value.join(" ")) {
        onSave(args);
      }
    }}
  />
);

const GroupsSection = ({draft, problemId}: PanelProps): ReactNode => {
  const {save, isSaving} = useSaveDraft(problemId);
  const [name, setName] = useState("");
  const [parent, setParent] = useState<string | null>(null);
  const rows = groupRows(draft);
  const maxPoints = groupMaxPoints(draft);
  const icpcChecker = [draft.checker, draft.interactor].some((p) => p?.protocol === "icpc");
  const icpcValidators = draft.validators.some((v) => v.protocol === "icpc");

  const update = (id: string, patch: Parameters<typeof updateGroup>[2]) =>
    save((current) => updateGroup(current, id, patch));

  return (
    <SectionPaper title="Группы тестов">
      <Stack gap="sm">
        <Text size="xs" c="dimmed">
          Группа складывает баллы своих тестов и подгрупп, взятых по порядку первых тестов: «всё или ничего» даёт
          минимум, то есть баллы, только если пройдено всё. «До первой ошибки» не учитывает то, что идёт после первого
          непройденного теста или подгруппы. Группа начисляет баллы, только если полностью пройдены группы, которые
          она требует.
        </Text>
        <Table withTableBorder>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Название</Table.Th>
              <Table.Th>Начисление</Table.Th>
              <Table.Th>До первой ошибки</Table.Th>
              <Table.Th>Видно на контесте</Table.Th>
              <Table.Th>Требует</Table.Th>
              {(icpcChecker || icpcValidators) && <Table.Th>Аргументы</Table.Th>}
              <Table.Th>Баллы</Table.Th>
              <Table.Th />
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {rows.map(({group, depth}) => (
              <Table.Tr key={group.id}>
                <Table.Td pl={8 + depth * 20}>
                  {depth === 0 ? (
                    <Text size="sm" c="dimmed">
                      {groupLabel(group)}
                    </Text>
                  ) : (
                    <TextInput
                      size="xs"
                      defaultValue={group.name}
                      onBlur={(e) => {
                        const name = e.currentTarget.value.trim();
                        if (name !== "" && name !== group.name) {
                          void update(group.id, {name});
                        }
                      }}
                    />
                  )}
                </Table.Td>
                <Table.Td>
                  <Select
                    size="xs"
                    data={AGGREGATES}
                    value={group.aggregate}
                    allowDeselect={false}
                    onChange={(value) => value && void update(group.id, {aggregate: value as ProblemDraftGroup["aggregate"]})}
                  />
                </Table.Td>
                <Table.Td>
                  <Checkbox
                    checked={group.stop_on_failure}
                    onChange={(e) => void update(group.id, {stop_on_failure: e.currentTarget.checked})}
                  />
                </Table.Td>
                <Table.Td>
                  <Select
                    size="xs"
                    data={FEEDBACK}
                    value={group.feedback}
                    allowDeselect={false}
                    onChange={(value) => value && void update(group.id, {feedback: value as ProblemDraftGroup["feedback"]})}
                  />
                </Table.Td>
                <Table.Td>
                  {depth > 0 && (
                    <MultiSelect
                      size="xs"
                      data={requirableGroups(draft, group.id).map((g) => ({value: g.id, label: groupLabel(g)}))}
                      value={group.requires}
                      placeholder="—"
                      onChange={(ids) => void update(group.id, {requires: ids})}
                    />
                  )}
                </Table.Td>
                {(icpcChecker || icpcValidators) && (
                  <Table.Td>
                    <Stack gap={4}>
                      {icpcChecker && <ArgsInput label="чекеру" value={group.checker_args} onSave={(args) => void update(group.id, {checker_args: args})} />}
                      {icpcValidators && (
                        <ArgsInput label="валидаторам" value={group.validator_args} onSave={(args) => void update(group.id, {validator_args: args})} />
                      )}
                    </Stack>
                  </Table.Td>
                )}
                <Table.Td>
                  <Stack gap={4}>
                    <Text size="sm">{formatPoints(maxPoints.get(group.id) ?? 0)}</Text>
                    <NumberInput
                      size="xs"
                      w={110}
                      placeholder="вес всех тестов"
                      min={0}
                      decimalScale={2}
                      hideControls
                      onBlur={(e) => {
                        const value = Number(e.currentTarget.value.replace(",", "."));
                        if (e.currentTarget.value !== "" && Number.isFinite(value) && value >= 0) {
                          void save((current) => setGroupTestPoints(current, group.id, Math.round(value * 100) / 100));
                          e.currentTarget.value = "";
                        }
                      }}
                    />
                  </Stack>
                </Table.Td>
                <Table.Td>
                  {depth > 0 && (
                    <Tooltip label="Удалить группу; её тесты перейдут в родительскую">
                      <ActionIcon variant="subtle" color="red" onClick={() => void save((current) => removeGroup(current, group.id))}>
                        <IconTrash size={16} />
                      </ActionIcon>
                    </Tooltip>
                  )}
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
        <Group align="flex-end">
          <TextInput label="Новая группа" placeholder="Подзадача 1" value={name} onChange={(e) => setName(e.currentTarget.value)} />
          <Select
            label="Внутри"
            data={rows.map(({group, depth}) => ({value: group.id, label: "\u2003".repeat(depth) + groupLabel(group)}))}
            value={parent ?? draft.testset.id}
            allowDeselect={false}
            onChange={setParent}
          />
          <Button
            variant="light"
            disabled={name.trim() === ""}
            loading={isSaving}
            onClick={async () => {
              if (await save((current) => addGroup(current, parent ?? current.testset.id, name.trim()))) {
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
  const command = test.input.steps.map((step) => [step.generator || "?", ...step.args].join(" ")).join(" | ");
  return (
    <Text size="sm" ff="monospace">
      {command}
      {test.input.file && ` → ${test.input.file}`}
    </Text>
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
  const rows = groupRows(draft);
  const groupNames = new Map(rows.map(({group}) => [group.id, groupLabel(group)]));

  const create = (kind: "manual" | "generator") => {
    const group = rows.at(-1)!.group;
    const test = kind === "manual" ? newManualTest(draft, group.id) : newGeneratedTest(draft, group.id);
    setEditing({test, isNew: true});
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
            <Button variant="light" onClick={() => create("manual")}>
              Тест из файлов
            </Button>
            <Button variant="light" onClick={() => create("generator")}>
              Сгенерированный тест
            </Button>
          </Group>
        </Group>
        {draft.tests.length > 0 && (
          <Table withTableBorder highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th w={50}>№</Table.Th>
                <Table.Th>Группа</Table.Th>
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
                      <Tooltip label="Не показывается в условии: задайте ввод для условия" disabled={!(isInteractive(draft.type) && test.statement_input === null)}>
                        <Badge ml={4} size="xs" variant="light" color={isInteractive(draft.type) && test.statement_input === null ? "yellow" : undefined}>
                          пример
                        </Badge>
                      </Tooltip>
                    )}
                    {test.pretest && (
                      <Badge ml={4} size="xs" variant="light" color="grape">
                        претест
                      </Badge>
                    )}
                  </Table.Td>
                  <Table.Td>{groupNames.get(test.group_id) ?? "?"}</Table.Td>
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
            <GroupsSection draft={draft} problemId={problemId} />
            <TestsSection draft={draft} problemId={problemId} />
          </>
        )}
      </DraftGate>
    </Stack>
  </Box>
);
