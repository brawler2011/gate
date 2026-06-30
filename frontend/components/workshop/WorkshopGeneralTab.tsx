"use client";

import { SectionPaper } from "@/components/workshop/SectionPaper";
import {
  getWorkshopProblemLimits,
  updateWorkshopProblemLimits,
} from "@/lib/actions";
import {
  Box,
  Button,
  Grid,
  Group,
  NumberInput,
  Select,
  Stack,
  Text,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useCallback, useEffect, useState, useTransition } from "react";

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
  const [isLoading, startLoading] = useTransition();
  const [isSaving, startSaving] = useTransition();
  const [isDirty, setIsDirty] = useState(false);

  const loadLimits = useCallback(() => {
    startLoading(async () => {
      const [error, data] = await getWorkshopProblemLimits(problemId);
      if (error || !data) {
        notifications.show({
          title: "Ошибка загрузки лимитов",
          message: error?.message ?? "Не удалось загрузить лимиты задачи",
          color: "red",
        });
        return;
      }

      setLimits({
        problem_type: data.problem_type,
        max_score: data.max_score ?? null,
        time_limit_ms: data.time_limit_ms,
        memory_limit_mb: data.memory_limit_mb,
      });
      setIsDirty(false);
    });
  }, [problemId, startLoading]);
  useEffect(() => {
    loadLimits();
  }, [loadLimits]);

  const patchLimits = (patch: Partial<LimitsData>) => {
    setLimits((prev) => (prev ? { ...prev, ...patch } : prev));
    setIsDirty(true);
  };

  const handleSave = () => {
    if (!limits) return;

    startSaving(async () => {
      const [error] = await updateWorkshopProblemLimits(problemId, {
        problem_type: limits.problem_type,
        max_score: limits.problem_type === "scoring" ? limits.max_score : null,
        time_limit_ms: limits.time_limit_ms,
        memory_limit_mb: limits.memory_limit_mb,
      });

      if (error) {
        notifications.show({
          title: "Ошибка сохранения",
          message: error.message ?? "Не удалось сохранить лимиты",
          color: "red",
        });
        return;
      }

      setIsDirty(false);
      notifications.show({
        title: "Сохранено",
        message: "Лимиты задачи обновлены",
        color: "green",
      });
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
