"use client";

import { SectionPaper } from "@/components/workshop/SectionPaper";
import {
  getProblem,
  getWorkshopProblemLimits,
  updateProblem,
  updateWorkshopProblemLimits,
} from "@/lib/actions";
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
import { notifications } from "@mantine/notifications";
import { useEffect, useState, useTransition } from "react";
import useSWR, { useSWRConfig } from "swr";

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
  { value: "pass-fail", label: "Pass-Fail" },
  { value: "scoring", label: "Scoring" },
  { value: "interactive", label: "Interactive" },
];

export function WorkshopGeneralTab({ problemId }: Props) {
  const [limits, setLimits] = useState<LimitsData | null>(null);
  const [isTemplate, setIsTemplate] = useState<boolean>(false);
  const [isSaving, startSaving] = useTransition();
  const [isDirty, setIsDirty] = useState(false);
  const { mutate: mutateGlobal } = useSWRConfig();

  const { data: problemData, isLoading: isLoadingProblem } = useSWR(
    ["problem", problemId],
    async () => {
      const [err, res] = await getProblem(problemId);
      if (err) throw err;
      return res;
    }
  );

  const { data: limitsData, isLoading: isLoadingLimits } = useSWR(
    ["problem-limits", problemId],
    async () => {
      const [err, res] = await getWorkshopProblemLimits(problemId);
      if (err) throw err;
      return res;
    }
  );

  const isLoading = isLoadingProblem || isLoadingLimits;

  useEffect(() => {
    if (limitsData) {
      setLimits({
        problem_type: limitsData.problem_type,
        max_score: limitsData.max_score ?? null,
        time_limit_ms: limitsData.time_limit_ms,
        memory_limit_mb: limitsData.memory_limit_mb,
      });
    }
  }, [limitsData]);

  useEffect(() => {
    if (problemData) {
      setIsTemplate(!!problemData.problem.is_template);
    }
  }, [problemData]);

  const patchLimits = (patch: Partial<LimitsData>) => {
    setLimits((prev) => (prev ? { ...prev, ...patch } : prev));
    setIsDirty(true);
  };

  const handleSave = () => {
    if (!limits) return;

    startSaving(async () => {
      const [limitsError] = await updateWorkshopProblemLimits(problemId, {
        problem_type: limits.problem_type,
        max_score: limits.problem_type === "scoring" ? limits.max_score : null,
        time_limit_ms: limits.time_limit_ms,
        memory_limit_mb: limits.memory_limit_mb,
      });

      if (limitsError) {
        notifications.show({
          title: "Ошибка сохранения",
          message: limitsError.message || "Не удалось сохранить лимиты",
          color: "red",
        });
        return;
      }

      const [problemError] = await updateProblem(problemId, {
        is_template: isTemplate,
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

      mutateGlobal(["problem", problemId]);
      mutateGlobal(["problem-limits", problemId]);
    });
  };

  return (
    <Box p="lg">
      <Stack gap="lg" maw={900} mx="auto">
        <SectionPaper title="Настройки задачи">
          {isLoading || !limits ? (
            <Text c="dimmed" size="sm">
              Загрузка...
            </Text>
          ) : (
            <Stack gap="md">
              <Grid gutter="md">
                <Grid.Col span={{ base: 12, sm: 4 }}>
                  <Select
                    label="Тип задачи"
                    description="Схема оценивания"
                    data={PROBLEM_TYPE_OPTIONS}
                    value={limits.problem_type}
                    onChange={(value) => {
                      if (!value) return;
                      patchLimits({
                        problem_type: value,
                        max_score:
                          value === "scoring" ? limits.max_score : null,
                      });
                    }}
                  />
                </Grid.Col>
                <Grid.Col span={{ base: 12, sm: 4 }}>
                  <NumberInput
                    label="Лимит времени"
                    description="В миллисекундах"
                    suffix=" мс"
                    min={0}
                    value={limits.time_limit_ms}
                    onChange={(value) =>
                      patchLimits({
                        time_limit_ms: typeof value === "number" ? value : 0,
                      })
                    }
                  />
                </Grid.Col>
                <Grid.Col span={{ base: 12, sm: 4 }}>
                  <NumberInput
                    label="Лимит памяти"
                    description="В мегабайтах"
                    suffix=" МБ"
                    min={0}
                    value={limits.memory_limit_mb}
                    onChange={(value) =>
                      patchLimits({
                        memory_limit_mb: typeof value === "number" ? value : 0,
                      })
                    }
                  />
                </Grid.Col>

                <Grid.Col span={{ base: 12, sm: 4 }}>
                  <NumberInput
                    label="Максимальный балл"
                    description="Только для scoring-задач"
                    min={0}
                    disabled={limits.problem_type !== "scoring"}
                    value={limits.max_score ?? ""}
                    onChange={(value) =>
                      patchLimits({
                        max_score: typeof value === "number" ? value : null,
                      })
                    }
                    placeholder="Не задан"
                  />
                </Grid.Col>

                <Grid.Col span={12}>
                  <Divider my="sm" />
                </Grid.Col>

                <Grid.Col span={12}>
                  <Switch
                    label="Использовать как шаблон"
                    description="При создании новых задач в текущей организации эту задачу можно будет выбрать в качестве шаблона. Для включения необходим хотя бы один успешно собранный (ready) пакет."
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
                  loading={isSaving}
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
}
