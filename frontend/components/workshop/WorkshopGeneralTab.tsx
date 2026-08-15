"use client";

import {
  Box,
  Button,
  Divider,
  Grid,
  Group,
  NumberInput,
  Select,
  Stack,
  Switch,
  Text,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {useEffect, useState, useTransition} from "react";
import useSWR, {useSWRConfig} from "swr";

import {SectionPaper} from "@/components/workshop/SectionPaper";
import {api} from "@/lib/api";

import type {ReactNode} from "react";

type LimitsData = {
  problem_type: string;
  max_score: number | null;
  time_limit_ms: number;
  memory_limit_mb: number;
};

type Props = {
  problemId: string;
};

const PROBLEM_TYPE_OPTIONS = [
  {value: "pass-fail", label: "Pass-Fail"},
  {value: "scoring", label: "Scoring"},
  {value: "interactive", label: "Interactive"},
];

export const WorkshopGeneralTab = ({problemId}: Props): ReactNode => {
  const {mutate} = useSWRConfig();
  const [limits, setLimits] = useState<LimitsData>({
    problem_type: "pass-fail",
    max_score: null,
    time_limit_ms: 2000,
    memory_limit_mb: 256,
  });
  const [isTemplate, setIsTemplate] = useState(false);
  const [isPending, startTransition] = useTransition();
  const [isDirty, setIsDirty] = useState(false);

  const {data: problemData, isLoading: isLoadingProblem} = useSWR(
    ["problem", problemId],
    async () => {
      const [err, res] = await api.getProblem({id: problemId});
      if (err) {
        throw new Error(err.message || "Не удалось загрузить задачу");
      }
      return res;
    }
  );

  const {data: limitsData, isLoading: isLoadingLimits} = useSWR(
    ["problem-limits", problemId],
    async () => {
      const [err, res] = await api.getProblemLimits({problemId});
      if (err) {
        throw new Error(err.message || "Не удалось загрузить ограничения");
      }
      return res;
    }
  );

  const isLoading = isLoadingProblem || isLoadingLimits;

  useEffect(() => {
    if (limitsData) {
      setLimits({
        problem_type: limitsData.problem_type ?? "pass-fail",
        max_score: limitsData.max_score ?? null,
        time_limit_ms: limitsData.time_limit_ms ?? 2000,
        memory_limit_mb: limitsData.memory_limit_mb ?? 256,
      });
    }
  }, [limitsData]);

  useEffect(() => {
    if (problemData?.problem) {
      setIsTemplate(problemData.problem.is_template ?? false);
    }
  }, [problemData]);

  const patchLimits = (patch: Partial<LimitsData>) => {
    setLimits((prev) => ({...prev, ...patch}));
    setIsDirty(true);
  };

  const handleSave = () => {
    startTransition(async () => {
      const [limitsError] = await api.updateProblemLimits({
        problemId,
        requestBody: {
          problem_type: limits.problem_type,
          max_score: limits.problem_type === "scoring" ? limits.max_score : null,
          time_limit_ms: limits.time_limit_ms,
          memory_limit_mb: limits.memory_limit_mb,
        },
      });

      if (limitsError) {
        notifications.show({
          title: "Ошибка сохранения",
          message: limitsError.message || "Не удалось сохранить лимиты",
          color: "red",
        });
        return;
      }

      const [problemError] = await api.updateProblem({
        id: problemId,
        requestBody: {
          is_template: isTemplate,
        },
      });

      if (problemError) {
        notifications.show({
          title: "Ошибка сохранения шаблона",
          message: problemError.message || "Не удалось обновить настройки шаблона",
          color: "red",
        });
        return;
      }

      setIsDirty(false);
      notifications.show({
        title: "Сохранено",
        message: "Настройки задачи обновлены",
        color: "green",
      });

      mutate(["problem", problemId]);
      mutate(["problem-limits", problemId]);
    });
  };

  return (
    <Box p="lg">
      <Stack gap="lg" maw={900} mx="auto">
        <SectionPaper title="Настройки задачи">
          {isLoading || !limits ? (
            <Text size="sm" c="dimmed">
              Загрузка настроек...
            </Text>
          ) : (
            <Stack gap="md">
              <Grid gutter="md">
                <Grid.Col span={{base: 12, sm: 6}}>
                  <Select
                    label="Тип задачи"
                    data={PROBLEM_TYPE_OPTIONS}
                    value={limits.problem_type}
                    onChange={(val) =>
                      patchLimits({problem_type: val || "pass-fail"})
                    }
                  />
                </Grid.Col>
                {limits.problem_type === "scoring" && (
                  <Grid.Col span={{base: 12, sm: 6}}>
                    <NumberInput
                      label="Максимальный балл"
                      value={limits.max_score ?? 100}
                      onChange={(val) =>
                        patchLimits({
                          max_score: typeof val === "number" ? val : null,
                        })
                      }
                      min={0}
                    />
                  </Grid.Col>
                )}
                <Grid.Col span={{base: 12, sm: 6}}>
                  <NumberInput
                    label="Ограничение по времени (мс)"
                    value={limits.time_limit_ms}
                    onChange={(val) =>
                      patchLimits({
                        time_limit_ms: typeof val === "number" ? val : 2000,
                      })
                    }
                    min={100}
                    max={60000}
                    step={100}
                  />
                </Grid.Col>
                <Grid.Col span={{base: 12, sm: 6}}>
                  <NumberInput
                    label="Ограничение по памяти (МБ)"
                    value={limits.memory_limit_mb}
                    onChange={(val) =>
                      patchLimits({
                        memory_limit_mb: typeof val === "number" ? val : 256,
                      })
                    }
                    min={16}
                    max={2048}
                    step={16}
                  />
                </Grid.Col>

                <Grid.Col span={12}>
                  <Divider my="xs" />
                  <Switch
                    label="Шаблонная задача (доступна при создании других задач)"
                    checked={isTemplate}
                    onChange={(event) => {
                      setIsTemplate(event.currentTarget.checked);
                      setIsDirty(true);
                    }}
                  />
                </Grid.Col>
              </Grid>

              <Group justify="flex-end">
                <Button
                  size="sm"
                  disabled={!isDirty}
                  loading={isPending}
                  onClick={handleSave}
                >
                  Сохранить настройки
                </Button>
              </Group>
            </Stack>
          )}
        </SectionPaper>
      </Stack>
    </Box>
  );
};
