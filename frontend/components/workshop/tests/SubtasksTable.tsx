"use client";

import {
  ActionIcon,
  Alert,
  Badge,
  Button,
  Card,
  Grid,
  Group,
  MultiSelect,
  NumberInput,
  Select,
  Stack,
  Text,
  TextInput,
  Tooltip,
} from "@mantine/core";
import { IconPlus, IconTrash } from "@tabler/icons-react";
import { useEffect, useState } from "react";
import { SubtaskDeleteModal } from "./SubtaskDeleteModal";
import {
  formatOrdinalsToRanges,
  parseOrdinalsFromRanges,
  SubtaskItem,
  TestItem,
} from "./types";

type Props = {
  subtasks: SubtaskItem[];
  tests: TestItem[];
  onChangeSubtasks: (newSubtasks: SubtaskItem[]) => void;
  onDeleteSubtaskWithTests?: (subtaskName: string, testIds: string[]) => void;
  maxScore: number | null;
  problemType: string;
};

type SubtaskItemRowProps = {
  st: SubtaskItem;
  idx: number;
  tests: TestItem[];
  problemType: string;
  availableSubtaskNames: string[];
  onUpdateSubtask: (patch: Partial<SubtaskItem>) => void;
  onDeleteRequest: () => void;
};

function SubtaskItemRow({
  st,
  idx,
  tests,
  problemType,
  availableSubtaskNames,
  onUpdateSubtask,
  onDeleteRequest,
}: SubtaskItemRowProps) {
  const assignedOrdinals = tests
    .filter((t) => st.testIds.includes(t.id))
    .map((t) => t.ordinal);
  const formattedRange = formatOrdinalsToRanges(assignedOrdinals);

  const [rangeInput, setRangeInput] = useState(formattedRange);
  const [isEditing, setIsEditing] = useState(false);

  useEffect(() => {
    if (!isEditing) {
      setRangeInput(formattedRange);
    }
  }, [formattedRange, isEditing]);

  const handleRangeChange = (val: string) => {
    setRangeInput(val);

    const ordinals = parseOrdinalsFromRanges(val);
    const matchedIds = tests
      .filter((t) => ordinals.includes(t.ordinal))
      .map((t) => t.id);

    onUpdateSubtask({ testIds: matchedIds });
  };

  const handleRangeBlur = () => {
    setIsEditing(false);
    const ordinals = parseOrdinalsFromRanges(rangeInput);
    const matchedIds = tests
      .filter((t) => ordinals.includes(t.ordinal))
      .map((t) => t.id);
    const validOrdinals = tests
      .filter((t) => matchedIds.includes(t.id))
      .map((t) => t.ordinal);
    setRangeInput(formatOrdinalsToRanges(validOrdinals));
  };

  const allOrdinalsInSystem = new Set(tests.map((t) => t.ordinal));
  const parsedOrdinals = parseOrdinalsFromRanges(rangeInput);
  const missingOrdinals = parsedOrdinals.filter((ord) => !allOrdinalsInSystem.has(ord));

  return (
    <Card key={st.name || idx} withBorder px="sm" py={6} radius="md" style={{ position: "relative" }}>
      <Tooltip label="Удалить сабтаск">
        <ActionIcon
          color="red"
          variant="subtle"
          size="md"
          onClick={onDeleteRequest}
          style={{ position: "absolute", top: 8, right: 8, zIndex: 2 }}
        >
          <IconTrash size={18} />
        </ActionIcon>
      </Tooltip>

      <Grid gutter="xs" align="flex-start">
        <Grid.Col span={{ base: 12, sm: 3 }}>
          <TextInput
            label="Имя сабтаска"
            size="xs"
            value={st.name}
            onChange={(e) =>
              onUpdateSubtask({ name: e.currentTarget.value.trim() })
            }
          />
        </Grid.Col>

        {problemType === "scoring" && (
          <Grid.Col span={{ base: 6, sm: 2 }}>
            <NumberInput
              label="Баллы"
              size="xs"
              min={0}
              value={st.points}
              onChange={(v) =>
                onUpdateSubtask({
                  points: typeof v === "number" ? v : 0,
                })
              }
            />
          </Grid.Col>
        )}

        <Grid.Col span={{ base: 6, sm: 3 }}>
          <Select
            label="Политика"
            size="xs"
            data={[
              { value: "complete", label: "За весь сабтаск (complete)" },
              { value: "each", label: "За каждый тест (each)" },
            ]}
            value={st.policy}
            onChange={(v) =>
              onUpdateSubtask({
                policy: (v as "complete" | "each") || "complete",
              })
            }
          />
        </Grid.Col>

        <Grid.Col span={{ base: 12, sm: problemType === "scoring" ? 4 : 6 }} style={{ paddingRight: 32 }}>
          <MultiSelect
            label="Зависимости"
            size="xs"
            data={availableSubtaskNames.filter((n) => n !== st.name)}
            value={st.dependencies}
            onChange={(deps) => onUpdateSubtask({ dependencies: deps })}
            placeholder="Выберите сабтаски"
          />
        </Grid.Col>

        <Grid.Col span={12}>
          <TextInput
            label="Диапазоны тестов (например: 1-5, 8, 10-12)"
            size="xs"
            value={rangeInput}
            onFocus={() => setIsEditing(true)}
            onChange={(e) => handleRangeChange(e.currentTarget.value)}
            onBlur={handleRangeBlur}
            placeholder="1-5, 8"
            error={
              missingOrdinals.length > 0
                ? `Отсутствуют тесты №: ${missingOrdinals.join(", ")}`
                : null
            }
          />
        </Grid.Col>
      </Grid>
    </Card>
  );
}

export function SubtasksTable({
  subtasks,
  tests,
  onChangeSubtasks,
  onDeleteSubtaskWithTests,
  maxScore,
  problemType,
}: Props) {
  const [deleteModalSubtask, setDeleteModalSubtask] = useState<SubtaskItem | null>(null);

  const totalPoints = subtasks.reduce((sum, s) => sum + s.points, 0);
  const targetScore = maxScore ?? 100;
  const isPointsMismatch = problemType === "scoring" && totalPoints !== targetScore;

  const availableSubtaskNames = subtasks.map((s) => s.name);

  const handleAddSubtask = () => {
    let newIndex = subtasks.length + 1;
    let name = `subtask${newIndex}`;
    while (subtasks.some((s) => s.name === name)) {
      newIndex++;
      name = `subtask${newIndex}`;
    }

    const newSubtask: SubtaskItem = {
      name,
      points: problemType === "scoring" ? 10 : 0,
      policy: "complete",
      dependencies: [],
      testIds: [],
    };

    onChangeSubtasks([...subtasks, newSubtask]);
  };

  const handleUpdateSubtask = (index: number, patch: Partial<SubtaskItem>) => {
    const next = [...subtasks];
    next[index] = { ...next[index], ...patch };
    onChangeSubtasks(next);
  };

  const confirmSubtaskDelete = (subtask: SubtaskItem, deleteTests: boolean) => {
    if (deleteTests && onDeleteSubtaskWithTests) {
      onDeleteSubtaskWithTests(subtask.name, subtask.testIds);
    } else {
      onChangeSubtasks(subtasks.filter((s) => s.name !== subtask.name));
    }
  };

  return (
    <Stack gap="xs">
      <Group justify="space-between" align="center">
        <Group gap="sm">
          <Text fw={600} size="md">
            Сабтаски (Группы тестов)
          </Text>
          <Badge variant="outline" size="md">
            Всего: {subtasks.length}
          </Badge>
          {problemType === "scoring" && (
            <Badge
              color={isPointsMismatch ? "yellow" : "green"}
              variant="light"
              size="md"
            >
              Сумма баллов: {totalPoints} / {targetScore}
            </Badge>
          )}
        </Group>

        <Button
          size="xs"
          leftSection={<IconPlus size={16} />}
          onClick={handleAddSubtask}
        >
          Добавить сабтаск
        </Button>
      </Group>

      {isPointsMismatch && (
        <Alert color="yellow" title="Предупреждение по баллам">
          Сумма баллов сабтасков ({totalPoints}) не совпадает с максимальным баллом задачи ({targetScore}).
        </Alert>
      )}

      {subtasks.length === 0 ? (
        <Card withBorder p="md" radius="md">
          <Text size="sm" c="dimmed" ta="center">
            Сабтаски не заданы. Все тесты будут относиться к общей проверке (Pass-Fail).
          </Text>
        </Card>
      ) : (
        <Stack gap="xs">
          {subtasks.map((st, idx) => (
            <SubtaskItemRow
              key={st.name || idx}
              st={st}
              idx={idx}
              tests={tests}
              problemType={problemType}
              availableSubtaskNames={availableSubtaskNames}
              onUpdateSubtask={(patch) => handleUpdateSubtask(idx, patch)}
              onDeleteRequest={() => setDeleteModalSubtask(st)}
            />
          ))}
        </Stack>
      )}

      {deleteModalSubtask && (
        <SubtaskDeleteModal
          opened={!!deleteModalSubtask}
          onClose={() => setDeleteModalSubtask(null)}
          subtaskName={deleteModalSubtask.name}
          testCount={deleteModalSubtask.testIds.length}
          onConfirm={(deleteTests) => confirmSubtaskDelete(deleteModalSubtask, deleteTests)}
        />
      )}
    </Stack>
  );
}
