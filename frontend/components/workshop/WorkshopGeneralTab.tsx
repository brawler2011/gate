"use client";

import {
  Box,
  Button,
  Grid,
  Group,
  NumberInput,
  ScrollArea,
  Select,
  Stack,
  Text,
  Textarea,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useEffect, useState, useTransition } from "react";
import { SectionPaper } from "@/components/workshop/SectionPaper";
import { getWorkshopFile, saveWorkshopFile } from "@/lib/actions";

type ManifestData = {
  last_updated?: string;
  problem_type: string;
  max_score: number | null;
  time_limit_ms: number;
  memory_limit_mb: number;
  stdout_limit_mb: number;
  code_size_limit_kb: number;
  statement?: Record<string, unknown>;
  meta_files?: unknown[];
};

type RawFileState = {
  content: string;
  isDirty: boolean;
  isLoading: boolean;
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
  // Manifest state
  const [manifest, setManifest] = useState<ManifestData | null>(null);
  const [isLoadingManifest, startLoadingManifest] = useTransition();
  const [isSavingManifest, startSavingManifest] = useTransition();
  const [isManifestDirty, setIsManifestDirty] = useState(false);

  // README state
  const [readme, setReadme] = useState<RawFileState>({
    content: "",
    isDirty: false,
    isLoading: false,
  });
  const [isSavingReadme, startSavingReadme] = useTransition();

  // Load all files on mount
  useEffect(() => {
    loadManifest();
    loadRawFile("README.md", setReadme);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [problemId]);

  const loadManifest = () => {
    startLoadingManifest(async () => {
      const [error, data] = await getWorkshopFile(problemId, "manifest.json");
      if (error) {
        notifications.show({
          title: "Ошибка загрузки manifest.json",
          message: error.message ?? "Не удалось загрузить манифест",
          color: "red",
        });
        return;
      }
      try {
        const parsed: ManifestData = JSON.parse(typeof data === "string" ? data : "{}");
        setManifest(parsed);
        setIsManifestDirty(false);
      } catch {
        notifications.show({
          title: "Ошибка парсинга manifest.json",
          message: "Файл содержит невалидный JSON",
          color: "red",
        });
      }
    });
  };

  const loadRawFile = (
    path: string,
    setter: React.Dispatch<React.SetStateAction<RawFileState>>
  ) => {
    setter((prev) => ({ ...prev, isLoading: true }));
    // We don't have a direct way to mark loading with useTransition per-file here,
    // so we use a simple approach with the setter
    getWorkshopFile(problemId, path).then(([error, data]) => {
      if (error && error.status !== 404) {
        notifications.show({
          title: `Ошибка загрузки ${path}`,
          message: error.message ?? "Не удалось загрузить файл",
          color: "red",
        });
        setter((prev) => ({ ...prev, isLoading: false }));
        return;
      }
      setter({
        content: typeof data === "string" ? data : "",
        isDirty: false,
        isLoading: false,
      });
    });
  };

  const handleSaveManifest = () => {
    if (!manifest) return;
    startSavingManifest(async () => {
      // Preserve fields we don't edit (statement, meta_files, last_updated)
      const [currentError, currentData] = await getWorkshopFile(problemId, "manifest.json");
      let existing: Record<string, unknown> = {};
      if (!currentError && typeof currentData === "string") {
        try {
          existing = JSON.parse(currentData);
        } catch {
          // ignore
        }
      }

      const updated = {
        ...existing,
        problem_type: manifest.problem_type,
        max_score: manifest.max_score,
        time_limit_ms: manifest.time_limit_ms,
        memory_limit_mb: manifest.memory_limit_mb,
        stdout_limit_mb: manifest.stdout_limit_mb,
        code_size_limit_kb: manifest.code_size_limit_kb,
      };

      const [error] = await saveWorkshopFile(
        problemId,
        "manifest.json",
        JSON.stringify(updated, null, 2)
      );
      if (error) {
        notifications.show({
          title: "Ошибка сохранения",
          message: error.message ?? "Не удалось сохранить manifest.json",
          color: "red",
        });
        return;
      }
      setIsManifestDirty(false);
      notifications.show({ title: "Сохранено", message: "manifest.json", color: "green" });
    });
  };

  const handleSaveRaw = (
    path: string,
    content: string,
    setter: React.Dispatch<React.SetStateAction<RawFileState>>,
    startSaving: (fn: () => Promise<void>) => void
  ) => {
    startSaving(async () => {
      const [error] = await saveWorkshopFile(problemId, path, content);
      if (error) {
        notifications.show({
          title: "Ошибка сохранения",
          message: error.message ?? `Не удалось сохранить ${path}`,
          color: "red",
        });
        return;
      }
      setter((prev) => ({ ...prev, isDirty: false }));
      notifications.show({ title: "Сохранено", message: path, color: "green" });
    });
  };

  const patchManifest = (patch: Partial<ManifestData>) => {
    setManifest((prev) => (prev ? { ...prev, ...patch } : prev));
    setIsManifestDirty(true);
  };

  return (
    <ScrollArea style={{ flex: 1 }} p="lg">
      <Stack gap="lg" maw={900} mx="auto">
        {/* Manifest form */}
        <SectionPaper title="Настройки задачи">
          {isLoadingManifest || !manifest ? (
            <Text c="dimmed" size="sm">
              Загрузка…
            </Text>
          ) : (
            <Stack gap="md">
              <Grid gutter="md">
                <Grid.Col span={{ base: 12, sm: 4 }}>
                  <Select
                    label="Тип задачи"
                    description="Схема оценивания"
                    data={PROBLEM_TYPE_OPTIONS}
                    value={manifest.problem_type}
                    onChange={(val) => val && patchManifest({ problem_type: val })}
                  />
                </Grid.Col>
                <Grid.Col span={{ base: 12, sm: 4 }}>
                  <NumberInput
                    label="Лимит времени"
                    description="В миллисекундах"
                    suffix=" мс"
                    min={0}
                    value={manifest.time_limit_ms}
                    onChange={(val) =>
                      patchManifest({ time_limit_ms: typeof val === "number" ? val : 0 })
                    }
                  />
                </Grid.Col>
                <Grid.Col span={{ base: 12, sm: 4 }}>
                  <NumberInput
                    label="Лимит памяти"
                    description="В мегабайтах"
                    suffix=" МБ"
                    min={0}
                    value={manifest.memory_limit_mb}
                    onChange={(val) =>
                      patchManifest({ memory_limit_mb: typeof val === "number" ? val : 0 })
                    }
                  />
                </Grid.Col>
                <Grid.Col span={{ base: 12, sm: 4 }}>
                  <NumberInput
                    label="Лимит вывода"
                    description="В мегабайтах"
                    suffix=" МБ"
                    min={0}
                    value={manifest.stdout_limit_mb}
                    onChange={(val) =>
                      patchManifest({ stdout_limit_mb: typeof val === "number" ? val : 0 })
                    }
                  />
                </Grid.Col>
                <Grid.Col span={{ base: 12, sm: 4 }}>
                  <NumberInput
                    label="Лимит размера кода"
                    description="В килобайтах"
                    suffix=" КБ"
                    min={0}
                    value={manifest.code_size_limit_kb}
                    onChange={(val) =>
                      patchManifest({ code_size_limit_kb: typeof val === "number" ? val : 0 })
                    }
                  />
                </Grid.Col>
                <Grid.Col span={{ base: 12, sm: 4 }}>
                  <NumberInput
                    label="Максимальный балл"
                    description="Только для scoring-задач"
                    min={0}
                    value={manifest.max_score ?? ""}
                    onChange={(val) =>
                      patchManifest({ max_score: typeof val === "number" ? val : null })
                    }
                    placeholder="Не задан"
                  />
                </Grid.Col>
              </Grid>
              <Group justify="flex-end">
                <Button
                  size="sm"
                  disabled={!isManifestDirty}
                  loading={isSavingManifest}
                  onClick={handleSaveManifest}
                >
                  Сохранить настройки
                </Button>
              </Group>
            </Stack>
          )}
        </SectionPaper>

        {/* README.md */}
        <SectionPaper title="README.md">
          <Stack gap="sm">
            {readme.isLoading ? (
              <Text c="dimmed" size="sm">
                Загрузка…
              </Text>
            ) : (
              <Box>
                <Textarea
                  value={readme.content}
                  onChange={(e) =>
                    setReadme((prev) => ({
                      ...prev,
                      content: e.currentTarget.value,
                      isDirty: true,
                    }))
                  }
                  minRows={8}
                  maxRows={20}
                  autosize
                  styles={{
                    input: {
                      fontFamily: "var(--mantine-font-family-monospace)",
                      fontSize: 13,
                    },
                  }}
                />
              </Box>
            )}
            <Group justify="flex-end">
              <Button
                size="sm"
                variant="default"
                disabled={!readme.isDirty}
                loading={isSavingReadme}
                onClick={() =>
                  handleSaveRaw("README.md", readme.content, setReadme, startSavingReadme)
                }
              >
                Сохранить README.md
              </Button>
            </Group>
          </Stack>
        </SectionPaper>

      </Stack>
    </ScrollArea>
  );
}
