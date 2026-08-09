"use client";

import {
  ActionIcon,
  Badge,
  Button,
  Card,
  Checkbox,
  Group,
  Menu,
  Popover,
  Select,
  Stack,
  Table,
  Text,
  TextInput,
  Tooltip,
} from "@mantine/core";
import {
  IconChevronDown,
  IconPlus,
  IconTrash,
} from "@tabler/icons-react";
import {useState} from "react";

import {TestPreviewModal} from "./TestPreviewModal";
import {TestVerdictModal} from "./TestVerdictModal";
import {formatPaddedOrdinal} from "./types";

import type {SubtaskItem, TestItem} from "./types";

type Props = {
  problemId: string;
  tests: TestItem[];
  subtasks: SubtaskItem[];
  generators: string[];
  onChangeTests: (newTests: TestItem[]) => void;
  onChangeSubtasks: (newSubtasks: SubtaskItem[]) => void;
  onAddTest: () => void;
  onDeleteSelectedTests: (ids: string[]) => void;
  onSavedPreviewFile?: () => void;
};

export const TestsListTable = ({
  problemId,
  tests,
  subtasks,
  generators,
  onChangeTests,
  onChangeSubtasks,
  onAddTest,
  onDeleteSelectedTests,
  onSavedPreviewFile,
}: Props) => {
  const [deletingTestId, setDeletingTestId] = useState<string | null>(null);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [previewFile, setPreviewFile] = useState<string | null>(null);
  const [verdictModalData, setVerdictModalData] = useState<{
    title: string;
    verdictBadge?: { label: string; color: string };
    time?: number;
    memory?: number;
    message?: string;
    error?: string;
  } | null>(null);

  const allSelected = tests.length > 0 && selectedIds.length === tests.length;
  const indeterminate = selectedIds.length > 0 && selectedIds.length < tests.length;

  const toggleSelectAll = () => {
    if (allSelected) {
      setSelectedIds([]);
    } else {
      setSelectedIds(tests.map((t) => t.id));
    }
  };

  const toggleSelect = (id: string) => {
    if (selectedIds.includes(id)) {
      setSelectedIds(selectedIds.filter((item) => item !== id));
    } else {
      setSelectedIds([...selectedIds, id]);
    }
  };

  const handleUpdateTest = (index: number, patch: Partial<TestItem>) => {
    const next = [...tests];
    next[index] = {...next[index], ...patch};
    onChangeTests(next);
  };

  const handleAssignSelectedToSubtask = (subtaskName: string) => {
    const targetSubtask = subtasks.find((s) => s.name === subtaskName);
    if (!targetSubtask) {
      return;
    }

    const newTestIds = Array.from(new Set([...targetSubtask.testIds, ...selectedIds]));
    const nextSubtasks = subtasks.map((s) =>
      s.name === subtaskName ? {...s, testIds: newTestIds} : s
    );
    onChangeSubtasks(nextSubtasks);
    setSelectedIds([]);
  };

  const generatorOptions = generators.map((g) => ({value: g, label: g}));

  return (
    <Stack gap="xs">
      <Group justify="space-between" align="center">
        <Group gap="sm">
          <Text fw={600} size="md">
            Тесты задачи
          </Text>
          <Badge variant="outline" size="md">
            Всего: {tests.length}
          </Badge>
          {selectedIds.length > 0 && (
            <Badge color="blue" size="md">
              Выбрано: {selectedIds.length}
            </Badge>
          )}
        </Group>

        <Group gap="xs">
          {selectedIds.length > 0 && (
            <>
              {subtasks.length > 0 && (
                <Menu shadow="md" width={200}>
                  <Menu.Target>
                    <Button size="xs" variant="default" rightSection={<IconChevronDown size={14} />}>
                      Назначить в сабтаск
                    </Button>
                  </Menu.Target>
                  <Menu.Dropdown>
                    <Menu.Label>Выберите сабтаск</Menu.Label>
                    {subtasks.map((st) => (
                      <Menu.Item
                        key={st.name}
                        onClick={() => handleAssignSelectedToSubtask(st.name)}
                      >
                        {st.name} ({st.points} б.)
                      </Menu.Item>
                    ))}
                  </Menu.Dropdown>
                </Menu>
              )}

              <Button
                size="xs"
                color="red"
                variant="light"
                leftSection={<IconTrash size={14} />}
                onClick={() => {
                  onDeleteSelectedTests(selectedIds);
                  setSelectedIds([]);
                }}
              >
                Удалить выбранные ({selectedIds.length})
              </Button>
            </>
          )}

          <Button
            size="xs"
            leftSection={<IconPlus size={16} />}
            onClick={onAddTest}
          >
            Добавить тест
          </Button>
        </Group>
      </Group>

      {tests.length === 0 ? (
        <Card withBorder p="xl" radius="md" style={{textAlign: "center"}}>
          <Text c="dimmed" size="sm" mb="md">
            Тесты еще не созданы. Добавьте ручной тест или настройте генератор.
          </Text>
          <Group justify="center">
            <Button
              size="sm"
              leftSection={<IconPlus size={16} />}
              onClick={onAddTest}
            >
              Создать первый тест
            </Button>
          </Group>
        </Card>
      ) : (
        <Table highlightOnHover withTableBorder withColumnBorders verticalSpacing={3}>
          <Table.Thead>
            <Table.Tr>
              <Table.Th style={{width: 40, textAlign: "center"}}>
                <Checkbox
                  checked={allSelected}
                  indeterminate={indeterminate}
                  onChange={toggleSelectAll}
                />
              </Table.Th>
              <Table.Th style={{width: 60}}>#</Table.Th>
              <Table.Th style={{width: 80, textAlign: "center"}}>Сэмпл</Table.Th>
              <Table.Th style={{width: 140}}>Метод</Table.Th>
              <Table.Th>Детали генератора / Файлы</Table.Th>
              <Table.Th style={{width: 140}}>Сабтаски</Table.Th>
              <Table.Th style={{width: 150, textAlign: "center"}}>Проверки</Table.Th>
              <Table.Th style={{width: 60, textAlign: "center"}} />
            </Table.Tr>
          </Table.Thead>

          <Table.Tbody>
            {tests.map((test, index) => {
              const paddedOrd = formatPaddedOrdinal(test.ordinal);
              const isSelected = selectedIds.includes(test.id);

              // Parse generator name and args from generatorCommand
              let genName = "";
              let genArgs = "";
              if (test.generatorCommand) {
                const parts = test.generatorCommand.trim().split(/\s+/);
                genName = parts[0] || "";
                genArgs = parts.slice(1).join(" ");
              }

              return (
                <Table.Tr key={test.id} bg={isSelected ? "var(--mantine-color-blue-light)" : undefined}>
                  <Table.Td style={{textAlign: "center"}}>
                    <Checkbox
                      checked={isSelected}
                      onChange={() => toggleSelect(test.id)}
                    />
                  </Table.Td>

                  <Table.Td>
                    <Text fw={600} size="sm">
                      {paddedOrd}
                    </Text>
                  </Table.Td>

                  <Table.Td style={{textAlign: "center"}}>
                    <Checkbox
                      checked={test.isSample}
                      onChange={(e) =>
                        handleUpdateTest(index, {isSample: e.currentTarget.checked})
                      }
                    />
                  </Table.Td>

                  <Table.Td>
                    <Select
                      size="xs"
                      data={[
                        {value: "manual", label: "Ручной"},
                        {value: "generated", label: "Генератор"},
                      ]}
                      value={test.method}
                      onChange={(v) =>
                        handleUpdateTest(index, {
                          method: (v as "manual" | "generated") || "manual",
                        })
                      }
                    />
                  </Table.Td>

                  <Table.Td>
                    {test.method === "generated" ? (
                      <Group gap="xs" wrap="nowrap">
                        <Select
                          size="xs"
                          placeholder="Генератор"
                          data={generatorOptions}
                          value={genName}
                          onChange={(val) => {
                            const newCmd = val ? `${val} ${genArgs}`.trim() : "";
                            handleUpdateTest(index, {generatorCommand: newCmd});
                          }}
                          style={{width: 140}}
                        />
                        <TextInput
                          size="xs"
                          placeholder="Аргументы CLI (например: 100 500 42)"
                          value={genArgs}
                          onChange={(e) => {
                            const newArgs = e.currentTarget.value;
                            const newCmd = genName ? `${genName} ${newArgs}`.trim() : newArgs;
                            handleUpdateTest(index, {generatorCommand: newCmd});
                          }}
                          style={{flex: 1}}
                        />
                      </Group>
                    ) : (
                      <Group gap="xs">
                        <Badge
                          variant={test.hasIn ? "light" : "outline"}
                          color={test.hasIn ? "blue" : "gray"}
                          size="xs"
                          style={{cursor: "pointer"}}
                          onClick={() => setPreviewFile(`${paddedOrd}.in`)}
                        >
                          {paddedOrd}.in {test.hasIn ? "✓" : "✗"}
                        </Badge>

                        <Badge
                          variant={test.hasOut ? "light" : "outline"}
                          color={test.hasOut ? "green" : "gray"}
                          size="xs"
                          style={{cursor: "pointer"}}
                          onClick={() => setPreviewFile(`${paddedOrd}.out`)}
                        >
                          {paddedOrd}.out {test.hasOut ? "✓" : "✗"}
                        </Badge>
                      </Group>
                    )}
                  </Table.Td>

                  <Table.Td>
                    <Group gap={4}>
                      {test.subtaskNames.length > 0 ? (
                        test.subtaskNames.map((name) => (
                          <Badge key={name} size="xs" variant="light" color="gray">
                            {name}
                          </Badge>
                        ))
                      ) : (
                        <Text size="xs" c="dimmed">
                          Не назначен
                        </Text>
                      )}
                    </Group>
                  </Table.Td>

                  <Table.Td style={{textAlign: "center"}}>
                    <Group gap={4} justify="center">
                      {test.validatorStatus && (
                        <Tooltip label="Результат проверки валидатором">
                          <Badge
                            size="xs"
                            color={test.validatorStatus.valid ? "green" : "red"}
                            style={{cursor: "pointer"}}
                            onClick={() =>
                              setVerdictModalData({
                                title: `Валидация теста №${paddedOrd}`,
                                verdictBadge: test.validatorStatus?.valid
                                  ? {label: "VALID", color: "green"}
                                  : {label: "INVALID", color: "red"},
                                message: test.validatorStatus?.message,
                                error: test.validatorStatus?.error,
                              })
                            }
                          >
                            Val: {test.validatorStatus.valid ? "OK" : "FAIL"}
                          </Badge>
                        </Tooltip>
                      )}

                      {test.solutionStatus && (
                        <Tooltip label="Результат эталонного решения">
                          <Badge
                            size="xs"
                            color={
                              test.solutionStatus.verdict === "OK"
                                ? "green"
                                : "red"
                            }
                            style={{cursor: "pointer"}}
                            onClick={() =>
                              setVerdictModalData({
                                title: `Решение на тесте №${paddedOrd}`,
                                verdictBadge: {
                                  label: test.solutionStatus?.verdict || "UNKNOWN",
                                  color:
                                    test.solutionStatus?.verdict === "OK" ? "green" : "red",
                                },
                                time: test.solutionStatus?.time,
                                memory: test.solutionStatus?.memory,
                                message: test.solutionStatus?.message,
                              })
                            }
                          >
                            Sol: {test.solutionStatus.verdict || "ERR"}
                          </Badge>
                        </Tooltip>
                      )}

                      {!test.validatorStatus && !test.solutionStatus && (
                        <Text size="xs" c="dimmed">
                          —
                        </Text>
                      )}
                    </Group>
                  </Table.Td>

                  <Table.Td style={{textAlign: "center"}}>
                    <Popover
                      opened={deletingTestId === test.id}
                      onChange={(opened) => setDeletingTestId(opened ? test.id : null)}
                      width={220}
                      position="left"
                      withArrow
                      shadow="md"
                      trapFocus
                    >
                      <Popover.Target>
                        <Tooltip label="Удалить тест">
                          <ActionIcon
                            size="xs"
                            variant="subtle"
                            color="red"
                            onClick={() =>
                              setDeletingTestId(deletingTestId === test.id ? null : test.id)
                            }
                          >
                            <IconTrash size={15} />
                          </ActionIcon>
                        </Tooltip>
                      </Popover.Target>
                      <Popover.Dropdown>
                        <Stack gap="xs">
                          <Text size="xs" fw={600}>
                            Удалить тест №{paddedOrd}?
                          </Text>
                          <Text size="xs" c="dimmed">
                            Файлы {paddedOrd}.in и {paddedOrd}.out будут удалены.
                          </Text>
                          <Group justify="flex-end" gap={6} mt={4}>
                            <Button
                              size="xs"
                              variant="default"
                              style={{height: 24, fontSize: 11}}
                              onClick={() => setDeletingTestId(null)}
                            >
                              Отмена
                            </Button>
                            <Button
                              size="xs"
                              color="red"
                              style={{height: 24, fontSize: 11}}
                              onClick={() => {
                                setDeletingTestId(null);
                                onDeleteSelectedTests([test.id]);
                              }}
                            >
                              Удалить
                            </Button>
                          </Group>
                        </Stack>
                      </Popover.Dropdown>
                    </Popover>
                  </Table.Td>
                </Table.Tr>
              );
            })}
          </Table.Tbody>
        </Table>
      )}

      {previewFile && (
        <TestPreviewModal
          opened={!!previewFile}
          onClose={() => setPreviewFile(null)}
          problemId={problemId}
          filename={previewFile}
          onSaved={onSavedPreviewFile}
        />
      )}

      {verdictModalData && (
        <TestVerdictModal
          opened={!!verdictModalData}
          onClose={() => setVerdictModalData(null)}
          {...verdictModalData}
        />
      )}
    </Stack>
  );
};
