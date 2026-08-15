"use client";

import {
  Alert,
  Box,
  Button,
  Group,
  LoadingOverlay,
  Paper,
  Stack,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {
  IconCheck,
  IconDeviceFloppy,
  IconPlayerPlay,
  IconRefresh,
  IconShieldCheck,
} from "@tabler/icons-react";
import {useEffect, useState, useTransition} from "react";
import useSWR from "swr";

import {api} from "@/lib/api";
import {
  createWorkshopTestFile,
  generateWorkshopTests,
  getWorkshopTestFile,
  testWorkshopSolution,
  updateWorkshopTestFile,
} from "@/lib/workshop";

import {SubtasksTable} from "./SubtasksTable";
import {TestRenumberModal} from "./TestRenumberModal";
import {TestsListTable} from "./TestsListTable";
import {
  formatPaddedOrdinal
} from "./types";

import type {
  SubtaskItem,
  TestItem} from "./types";
import type {ReactNode} from "react";

type Props = {
  problemId: string;
};

export const WorkshopTestsManager = ({problemId}: Props): ReactNode => {
  const [tests, setTests] = useState<TestItem[]>([]);
  const [subtasks, setSubtasks] = useState<SubtaskItem[]>([]);
  const [generators, setGenerators] = useState<string[]>([]);
  const [solutions, setSolutions] = useState<string[]>([]);

  const [isLoading, setIsLoading] = useState(true);
  const [isSaving, startSaving] = useTransition();
  const [isDirty, setIsDirty] = useState(false);

  const [isRenumberModalOpen, setIsRenumberModalOpen] = useState(false);
  const [isBatchRunning, setIsBatchRunning] = useState(false);

  // Load problem limits to know problem_type and max_score
  const {data: limitsData} = useSWR(["problem-limits", problemId], async () => {
    const [err, res] = await api.getProblemLimits({problemId});
    if (err) {
      return null;
    }
    return res;
  });

  const problemType = limitsData?.problem_type || "pass-fail";
  const maxScore = limitsData?.max_score ?? null;

  // Load initial data
  const loadData = async () => {
    setIsLoading(true);
    try {
      const [_genErr, genRes] = await api.listProblemGenerators({problemId});
      if (genRes?.files) {
        setGenerators(genRes.files.map((f: { path?: string; name?: string }) => (f.path || f.name || "").split("/").pop() || ""));
      }

      const [_solErr, solRes] = await api.listProblemWorkshopSubmissions({problemId});
      if (solRes?.files) {
        setSolutions(solRes.files.map((f: { path?: string; name?: string }) => (f.path || f.name || "").split("/").pop() || ""));
      }

      const [_filesErr, filesRes] = await api.listProblemTests({problemId});
      const existingFiles = (filesRes?.files?.map((f: { path?: string; name?: string }) => f.path || f.name || "") || []).map(
        (p: string) => p.split("/").pop() || p
      );

      // Fetch tests config
      const [_configErr, configText] = await getWorkshopTestFile(problemId, "tests.json");
      let configData: { groups?: Record<string, unknown>[]; tests?: Record<string, unknown>[] } = {};
      if (!_configErr && configText) {
        try {
          configData = JSON.parse(configText);
        } catch {
          configData = {};
        }
      }

      const rawGroups = configData.groups || [];
      const rawTests = configData.tests || [];

      // Determine ordinals of existing test files (e.g. 01.in -> 1)
      const ordinalsSet = new Set<number>();
      for (const name of existingFiles) {
        const match = name.match(/^(\d+)\.(in|out)$/);
        if (match) {
          ordinalsSet.add(parseInt(match[1], 10));
        }
      }

      for (const rt of rawTests) {
        if (typeof rt.ordinal === "number") {
          ordinalsSet.add(rt.ordinal);
        }
      }

      const sortedOrdinals = Array.from(ordinalsSet).sort((a, b) => a - b);

      // Build SubtaskItems
      const loadedSubtasks: SubtaskItem[] = rawGroups.map((grp: Record<string, unknown>, idx: number) => {
        const testsArr = grp.tests as number[] | undefined;
        const start = testsArr?.[0] || 1;
        const end = testsArr?.[1] || 1;
        const testIds: string[] = [];

        for (let ord = start; ord <= end; ord++) {
          testIds.push(`test-id-${ord}`);
        }

        const dependsArr = grp.depends_on as unknown[] | undefined;

        return {
          name: (grp.name as string) || `subtask${idx + 1}`,
          points: (grp.points as number) ?? 10,
          policy: grp.points_policy === "each-test" || grp.policy === "each" ? "each" : "complete",
          dependencies: dependsArr ? dependsArr.map((d: unknown) => (typeof d === "string" ? d : `subtask${d}`)) : [],
          testIds,
        };
      });

      // Build TestItems
      const loadedTests: TestItem[] = sortedOrdinals.map((ord) => {
        const padded = formatPaddedOrdinal(ord);
        const rawT = rawTests.find((t: Record<string, unknown>) => t.ordinal === ord);

        const hasIn = existingFiles.includes(`${padded}.in`);
        const hasOut = existingFiles.includes(`${padded}.out`);

        const subtaskNames = loadedSubtasks
          .filter((st) => st.testIds.includes(`test-id-${ord}`))
          .map((st) => st.name);

        return {
          id: `test-id-${ord}`,
          ordinal: ord,
          filename: `${padded}.in`,
          outFilename: `${padded}.out`,
          hasIn,
          hasOut,
          isSample: !!rawT?.is_sample,
          method: rawT?.method === "generated" ? "generated" : "manual",
          generatorCommand: (rawT?.generator as string) || "",
          subtaskNames,
        };
      });

      setSubtasks(loadedSubtasks);
      setTests(loadedTests);
      setIsDirty(false);
    } catch (e: unknown) {
      notifications.show({
        title: "Ошибка загрузки тестов",
        message: (e as Error).message || "Не удалось загрузить данные тестов",
        color: "red",
      });
    } finally {
      setIsLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, [problemId]);

  // Check for gaps in ordinals (e.g. [1, 2, 4])
  const gapNumbers: number[] = [];
  if (tests.length > 0) {
    const maxOrd = Math.max(...tests.map((t) => t.ordinal));
    const ordSet = new Set(tests.map((t) => t.ordinal));
    for (let i = 1; i <= maxOrd; i++) {
      if (!ordSet.has(i)) {
        gapNumbers.push(i);
      }
    }
  }

  const handleSubtasksChange = (newSubtasks: SubtaskItem[]) => {
    setSubtasks(newSubtasks);
    setIsDirty(true);

    // Update subtaskNames on tests
    setTests((prev) =>
      prev.map((t) => ({
        ...t,
        subtaskNames: newSubtasks
          .filter((st) => st.testIds.includes(t.id))
          .map((st) => st.name),
      }))
    );
  };

  const handleTestsChange = (newTests: TestItem[]) => {
    setTests(newTests);
    setIsDirty(true);
  };

  const handleAddTest = async () => {
    const nextOrd = tests.length > 0 ? Math.max(...tests.map((t) => t.ordinal)) + 1 : 1;
    const padded = formatPaddedOrdinal(nextOrd);
    const newId = `test-id-${nextOrd}`;
    const inFilename = `${padded}.in`;
    const outFilename = `${padded}.out`;

    // Automatically create empty .in and .out files for the manual test
    await createWorkshopTestFile(problemId, inFilename, "");
    await createWorkshopTestFile(problemId, outFilename, "");

    const newTest: TestItem = {
      id: newId,
      ordinal: nextOrd,
      filename: inFilename,
      outFilename: outFilename,
      hasIn: true,
      hasOut: true,
      isSample: false,
      method: "manual",
      subtaskNames: [],
    };

    setTests([...tests, newTest]);
    setIsDirty(true);
  };

  const handleDeleteSelectedTests = (idsToDelete: string[]) => {
    const nextTests = tests.filter((t) => !idsToDelete.includes(t.id));
    const nextSubtasks = subtasks.map((st) => ({
      ...st,
      testIds: st.testIds.filter((id) => !idsToDelete.includes(id)),
    }));

    setTests(nextTests);
    setSubtasks(nextSubtasks);
    setIsDirty(true);

    notifications.show({
      title: "Тесты удалены из списка",
      message: `Удалено тестов: ${idsToDelete.length}. Нажмите «Сохранить», чтобы синхронизировать конфигурацию.`,
      color: "blue",
    });
  };

  const handleDeleteSubtaskWithTests = (subtaskName: string, testIdsToDelete: string[]) => {
    const nextSubtasks = subtasks.filter((st) => st.name !== subtaskName);
    const nextTests = tests.filter((t) => !testIdsToDelete.includes(t.id));

    setSubtasks(nextSubtasks);
    setTests(nextTests);
    setIsDirty(true);
  };

  // Perform full renumbering of tests to 1..N and update subtasks
  const handlePerformRenumbering = () => {
    const renumberedTests = tests.map((t, idx) => {
      const newOrd = idx + 1;
      const padded = formatPaddedOrdinal(newOrd);
      return {
        ...t,
        id: `test-id-${newOrd}`,
        ordinal: newOrd,
        filename: `${padded}.in`,
        outFilename: `${padded}.out`,
      };
    });

    setTests(renumberedTests);
    setIsDirty(true);
    setIsRenumberModalOpen(false);

    notifications.show({
      title: "Перенумерация выполнена",
      message: "Тесты приведены к порядку 1..N. Нажмите «Сохранить» для применения на сервере.",
      color: "green",
    });
  };

  // Save changes to backend
  const handleSave = () => {
    startSaving(async () => {
      try {
        // Ensure all manual tests have physical .in and .out files on disk
        for (const t of tests) {
          if (t.method === "manual") {
            if (!t.hasIn) {
              await createWorkshopTestFile(problemId, t.filename, "");
              t.hasIn = true;
            }
            if (!t.hasOut) {
              await createWorkshopTestFile(problemId, t.outFilename, "");
              t.hasOut = true;
            }
          }
        }

        type PayloadGroup = {
          ordinal: number;
          name: string;
          points: number;
          points_policy: string;
          depends_on: string[];
          tests: number[];
        };

        let payloadGroups: PayloadGroup[] = subtasks
          .map((st, idx): PayloadGroup | null => {
            const stTests = tests.filter(
              (t) => st.testIds.includes(t.id) || t.subtaskNames.includes(st.name)
            );
            const ordinals = stTests.map((t) => t.ordinal).sort((a, b) => a - b);
            if (ordinals.length === 0) {
              return null;
            }

            return {
              ordinal: idx + 1,
              name: st.name,
              points: st.points,
              points_policy: st.policy === "each" ? "each-test" : "complete-group",
              depends_on: st.dependencies,
              tests: [ordinals[0], ordinals[ordinals.length - 1]],
            };
          })
          .filter((g): g is PayloadGroup => g !== null);

        // If subtasks is empty or no subtask covers tests, but tests exist, create a default subtask covering all tests
        if (payloadGroups.length === 0 && tests.length > 0) {
          const sortedOrds = tests.map((t) => t.ordinal).sort((a, b) => a - b);
          payloadGroups = [
            {
              ordinal: 1,
              name: "subtask1",
              points: maxScore ?? 100,
              points_policy: problemType === "scoring" ? "each-test" : "complete-group",
              depends_on: [],
              tests: [sortedOrds[0], sortedOrds[sortedOrds.length - 1]],
            },
          ];
        }

        // Ensure all tests are covered by groups so backend problem.yaml doesn't drop unassigned tests
        if (payloadGroups.length > 0 && tests.length > 0) {
          const covered = new Set<number>();
          for (const g of payloadGroups) {
            for (let ord = g.tests[0]; ord <= g.tests[1]; ord++) {
              covered.add(ord);
            }
          }

          const unassigned = tests
            .map((t) => t.ordinal)
            .filter((ord) => !covered.has(ord))
            .sort((a, b) => a - b);

          if (unassigned.length > 0) {
            const lastGroup = payloadGroups[payloadGroups.length - 1];
            lastGroup.tests[0] = Math.min(lastGroup.tests[0], unassigned[0]);
            lastGroup.tests[1] = Math.max(lastGroup.tests[1], unassigned[unassigned.length - 1]);
          }
        }

        const payloadTests = tests.map((t) => ({
          ordinal: t.ordinal,
          method: t.method,
          generator: t.method === "generated" && t.generatorCommand ? t.generatorCommand : null,
          is_sample: t.isSample,
        }));

        const testsConfig = {
          groups: payloadGroups,
          tests: payloadTests,
        };

        const [err] = await updateWorkshopTestFile(problemId, "tests.json", JSON.stringify(testsConfig, null, 2));
        if (err) {
          notifications.show({
            title: "Ошибка сохранения",
            message: err.message || "Не удалось сохранить конфигурацию тестов",
            color: "red",
          });
          return;
        }

        notifications.show({
          title: "Сохранено",
          message: "Конфигурация тестов и сабтасков успешно обновлена",
          color: "green",
        });

        setIsDirty(false);
        loadData();
      } catch (e: unknown) {
        notifications.show({
          title: "Ошибка сохранения",
          message: (e as Error).message || "Произошла ошибка при сохранении",
          color: "red",
        });
      }
    });
  };

  // Batch action: Generate tests
  const handleRunGeneratorBatch = async () => {
    const generatedTests = tests.filter((t) => t.method === "generated" && t.generatorCommand);
    if (generatedTests.length === 0) {
      notifications.show({
        title: "Нет сгенерированных тестов",
        message: "Назначьте тип «Генератор» и укажите команду хотя бы для одного теста",
        color: "yellow",
      });
      return;
    }

    setIsBatchRunning(true);
    try {
      for (const t of generatedTests) {
        const parts = t.generatorCommand!.trim().split(/\s+/);
        const genName = parts[0];
        const genArgs = parts.slice(1);

        const [genErr] = await generateWorkshopTests(problemId, genName, [t.ordinal], [genArgs]);
        if (genErr) {
          notifications.show({
            title: `Ошибка генерации теста №${t.ordinal}`,
            message: genErr.message || "Не удалось сгенерировать тест",
            color: "red",
          });
        }
      }

      notifications.show({
        title: "Генерация завершена",
        message: `Успешно сгенерировано тестов: ${generatedTests.length}`,
        color: "green",
      });
      loadData();
    } catch (e: unknown) {
      notifications.show({
        title: "Ошибка генерации",
        message: (e as Error).message || "Ошибка при вызове генератора",
        color: "red",
      });
    } finally {
      setIsBatchRunning(false);
    }
  };

  // Batch action: Run Solution for .out answers
  const handleRunSolutionBatch = async () => {
    if (solutions.length === 0) {
      notifications.show({
        title: "Нет решений",
        message: "Добавьте хотя бы одно решение в папку solutions/",
        color: "yellow",
      });
      return;
    }

    setIsBatchRunning(true);
    try {
      const solutionPath = solutions[0];
      const [err, report] = await testWorkshopSolution(problemId, solutionPath);

      if (err || !report) {
        notifications.show({
          title: "Ошибка решения",
          message: err?.message || "Не удалось получить отчет о прогоне решения",
          color: "red",
        });
        return;
      }

      // Update tests solutionStatus
      if (report?.results) {
        setTests((prev) =>
          prev.map((t) => {
            const res = (report.results as Record<string, unknown>[])?.find((r) => r.test_number === t.ordinal);
            if (!res) {
              return t;
            }
            return {
              ...t,
              solutionStatus: {
                verdict: res.verdict as string,
                time: res.time as number,
                memory: res.memory as number,
                message: res.message as string,
              },
            };
          })
        );
      }

      notifications.show({
        title: "Прогон решения завершен",
        message: `Решение ${solutionPath} проверено. Пройдено тестов: ${report.passed_tests || 0} / ${report.total_tests || 0}`,
        color: "green",
      });
    } catch (e: unknown) {
      notifications.show({
        title: "Ошибка решения",
        message: (e as Error).message || "Не удалось запустить прогон решения",
        color: "red",
      });
    } finally {
      setIsBatchRunning(false);
    }
  };

  // Batch action: Run Validator
  const handleRunValidatorBatch = async () => {
    setIsBatchRunning(true);
    try {
      const [err, report] = await api.validateAllTests({problemId});
      if (err || !report) {
        notifications.show({
          title: "Ошибка валидации",
          message: err?.message || "Не удалось выполнить проверку валидатором",
          color: "red",
        });
        return;
      }

      if (report?.results) {
        setTests((prev) =>
          prev.map((t) => {
            const res = (report.results as Record<string, unknown>[])?.find((r) => r.test_number === t.ordinal);
            if (!res) {
              return t;
            }
            return {
              ...t,
              validatorStatus: {
                valid: res.valid as boolean,
                message: res.message as string,
                error: res.error as string,
              },
            };
          })
        );
      }

      notifications.show({
        title: "Валидация завершена",
        message: `Валидных тестов: ${report.valid_tests || 0} / ${report.total_tests || 0}`,
        color: report.invalid_tests === 0 ? "green" : "red",
      });
    } catch (e: unknown) {
      notifications.show({
        title: "Ошибка валидации",
        message: (e as Error).message || "Не удалось запустить валидатор",
        color: "red",
      });
    } finally {
      setIsBatchRunning(false);
    }
  };

  if (isLoading) {
    return (
      <Box p="xl" pos="relative" mih={300}>
        <LoadingOverlay visible />
      </Box>
    );
  }

  return (
    <Box p="md">
      <Stack gap="md" maw={1200} mx="auto">
        {/* Top Actions Hotbar */}
        <Paper withBorder p="md" radius="md">
          <Group justify="space-between" align="center" wrap="wrap">
            <Group gap="xs">
              <Button
                size="sm"
                color="blue"
                leftSection={<IconDeviceFloppy size={18} />}
                disabled={!isDirty}
                loading={isSaving}
                onClick={handleSave}
              >
                Сохранить изменения
              </Button>

              {gapNumbers.length > 0 && (
                <Button
                  size="sm"
                  variant="light"
                  color="orange"
                  leftSection={<IconRefresh size={18} />}
                  onClick={() => setIsRenumberModalOpen(true)}
                >
                  Перенумеровать тесты (1..N)
                </Button>
              )}
            </Group>

            <Group gap="xs">
              <Button
                size="sm"
                variant="outline"
                leftSection={<IconPlayerPlay size={16} />}
                loading={isBatchRunning}
                onClick={handleRunGeneratorBatch}
              >
                Сгенерировать тесты
              </Button>

              <Button
                size="sm"
                variant="outline"
                color="green"
                leftSection={<IconCheck size={16} />}
                loading={isBatchRunning}
                onClick={handleRunSolutionBatch}
              >
                Сгенерировать ответы (.out)
              </Button>

              <Button
                size="sm"
                variant="outline"
                color="violet"
                leftSection={<IconShieldCheck size={16} />}
                loading={isBatchRunning}
                onClick={handleRunValidatorBatch}
              >
                Проверить валидатором
              </Button>
            </Group>
          </Group>
        </Paper>

        {gapNumbers.length > 0 && (
          <Alert color="orange" title="Обнаружены пропуски в нумерации тестов">
            В списке тестов отсутствуют номера: <b>{gapNumbers.join(", ")}</b>. Вы можете нажать «Перенумеровать тесты (1..N)» в любой момент для приведения порядка тестов к непрерывной последовательности.
          </Alert>
        )}

        {/* Subtasks Section */}
        <SubtasksTable
          subtasks={subtasks}
          tests={tests}
          onChangeSubtasks={handleSubtasksChange}
          onDeleteSubtaskWithTests={handleDeleteSubtaskWithTests}
          maxScore={maxScore}
          problemType={problemType}
        />

        {/* Tests List Section */}
        <TestsListTable
          problemId={problemId}
          tests={tests}
          subtasks={subtasks}
          generators={generators}
          onChangeTests={handleTestsChange}
          onChangeSubtasks={handleSubtasksChange}
          onAddTest={handleAddTest}
          onDeleteSelectedTests={handleDeleteSelectedTests}
          onSavedPreviewFile={loadData}
        />
      </Stack>

      <TestRenumberModal
        opened={isRenumberModalOpen}
        onClose={() => setIsRenumberModalOpen(false)}
        onConfirm={handlePerformRenumbering}
        gapNumbers={gapNumbers}
      />
    </Box>
  );
};
