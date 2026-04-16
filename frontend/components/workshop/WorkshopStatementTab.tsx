"use client";

import { SectionPaper } from "@/components/workshop/SectionPaper";
import {
  getWorkshopProblemLimits,
  getWorkshopProblemStatement,
  updateWorkshopProblemStatement,
} from "@/lib/actions";
import {
  Box,
  Button,
  Group,
  ScrollArea,
  Stack,
  Text,
  TextInput,
  Textarea,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import "katex/dist/katex.min.css";
import { useDeferredValue, useEffect, useState, useTransition } from "react";
import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import "../problems/Problem.css";
import classes from "./WorkshopStatementTab.module.css";

type StatementData = {
  title: string;
  legend: string;
  input_format: string;
  output_format: string;
  notes: string;
  interaction: string;
  scoring: string;
};

type PreviewMeta = {
  problem_type?: string;
  max_score?: number | null;
  time_limit_ms?: number;
  memory_limit_mb?: number;
};

type LoadedPreviewMeta = {
  problem_type: string;
  max_score: number | null;
  time_limit_ms: number;
  memory_limit_mb: number;
};

type Props = {
  problemId: string;
};

function prettifyTimeLimit(timeLimit: number) {
  if (timeLimit % 1000 === 0) {
    return `${timeLimit / 1000} сек`;
  }

  return `${timeLimit} мс`;
}

function prettifyMemoryLimit(memoryLimit: number) {
  if (memoryLimit % 1000 === 0) {
    return `${memoryLimit / 1000} ГБ`;
  }

  return `${memoryLimit} МБ`;
}

function hasPreviewMeta(meta: PreviewMeta | null): meta is LoadedPreviewMeta {
  return (
    meta?.problem_type !== undefined &&
    meta.max_score !== undefined &&
    meta.time_limit_ms !== undefined &&
    meta.memory_limit_mb !== undefined
  );
}

function MarkdownBlock({ value }: { value: string }) {
  return (
    <div className="content">
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[rehypeKatex]}
      >
        {value}
      </ReactMarkdown>
    </div>
  );
}

function PreviewSection({ title, value }: { title: string; value: string }) {
  if (!value.trim()) return null;

  return (
    <Stack gap="xs">
      <Title order={3} className={classes.sectionTitle}>
        {title}
      </Title>
      <MarkdownBlock value={value} />
    </Stack>
  );
}

function WorkshopStatementPreview({
  statement,
  previewMeta,
}: {
  statement: StatementData;
  previewMeta: LoadedPreviewMeta;
}) {
  const hasContent = [
    statement.legend,
    statement.input_format,
    statement.output_format,
    statement.notes,
    statement.scoring,
  ].some((value) => value.trim());

  return (
    <Stack className="container" gap="md">
      <Stack align="center" gap={0} w="fit-content" mx="auto" mb="sm">
        <Title
          order={2}
        >{`A. ${statement.title.trim() || "Без названия"}`}</Title>
        <Stack align="center" gap={0}>
          <Text>
            ограничение по времени:{" "}
            {prettifyTimeLimit(previewMeta.time_limit_ms)}
          </Text>
          <Text>
            ограничение по памяти:{" "}
            {prettifyMemoryLimit(previewMeta.memory_limit_mb)}
          </Text>
          {previewMeta.problem_type === "scoring" &&
          previewMeta.max_score !== null ? (
            <Text>максимальный балл: {previewMeta.max_score}</Text>
          ) : null}
        </Stack>
      </Stack>

      {hasContent ? (
        <>
          {statement.legend.trim() ? (
            <MarkdownBlock value={statement.legend} />
          ) : null}
          <PreviewSection
            title="Входные данные"
            value={statement.input_format}
          />
          <PreviewSection
            title="Выходные данные"
            value={statement.output_format}
          />
          <PreviewSection title="Система оценки" value={statement.scoring} />
          <PreviewSection title="Примечание" value={statement.notes} />
        </>
      ) : (
        <Text c="dimmed" ta="center">
          Начни заполнять поля слева, и здесь появится preview условия.
        </Text>
      )}
    </Stack>
  );
}

export function WorkshopStatementTab({ problemId }: Props) {
  const [statement, setStatement] = useState<StatementData | null>(null);
  const [previewMeta, setPreviewMeta] = useState<PreviewMeta | null>(null);
  const [isDirty, setIsDirty] = useState(false);
  const [isLoading, startLoading] = useTransition();
  const [isSaving, startSaving] = useTransition();
  const deferredStatement = useDeferredValue(statement);

  useEffect(() => {
    startLoading(async () => {
      const [[limitsError, limits], [statementError, statementData]] =
        await Promise.all([
          getWorkshopProblemLimits(problemId),
          getWorkshopProblemStatement(problemId),
        ]);

      if (limitsError || !limits) {
        notifications.show({
          title: "Ошибка загрузки лимитов",
          message:
            limitsError?.message ?? "Не удалось загрузить лимиты для preview",
          color: "red",
        });
        return;
      }

      if (statementError || !statementData) {
        notifications.show({
          title: "Ошибка загрузки условия",
          message: statementError?.message ?? "Не удалось загрузить statement",
          color: "red",
        });
        return;
      }

      setPreviewMeta({
        problem_type: limits.problem_type,
        max_score: limits.max_score ?? null,
        time_limit_ms: limits.time_limit_ms,
        memory_limit_mb: limits.memory_limit_mb,
      });
      setStatement({
        title: statementData.title ?? "",
        legend: statementData.legend ?? "",
        input_format: statementData.input_format ?? "",
        output_format: statementData.output_format ?? "",
        notes: statementData.notes ?? "",
        interaction: statementData.interaction ?? "",
        scoring: statementData.scoring ?? "",
      });
      setIsDirty(false);
    });
  }, [problemId]);

  const patchStatement = (patch: Partial<StatementData>) => {
    setStatement((prev) => ({
      title: "",
      legend: "",
      input_format: "",
      output_format: "",
      notes: "",
      interaction: "",
      scoring: "",
      ...prev,
      ...patch,
    }));
    setIsDirty(true);
  };

  const handleSave = () => {
    startSaving(async () => {
      if (!statement) return;

      const [saveError] = await updateWorkshopProblemStatement(problemId, {
        title: statement.title,
        legend: statement.legend,
        input_format: statement.input_format,
        output_format: statement.output_format,
        notes: statement.notes,
        interaction: statement.interaction,
        scoring: statement.scoring,
      });

      if (saveError) {
        notifications.show({
          title: "Ошибка сохранения",
          message: saveError.message ?? "Не удалось сохранить условие",
          color: "red",
        });
        return;
      }

      setIsDirty(false);
      notifications.show({
        title: "Сохранено",
        message: "Условие задачи обновлено",
        color: "green",
      });
    });
  };

  return (
    <Box className={classes.root}>
      <Box className={classes.editorPane}>
        <ScrollArea style={{ height: "100%" }} p="lg">
          <Stack gap="lg" maw={900} mx="auto">
            <SectionPaper title="Условие задачи (statement)">
              {isLoading ? (
                <Text c="dimmed" size="sm">
                  Загрузка...
                </Text>
              ) : (
                <Stack gap="md">
                  {!statement ? null : (
                    <>
                      <TextInput
                        label="Заголовок"
                        value={statement.title}
                        onChange={(e) =>
                          patchStatement({ title: e.currentTarget.value })
                        }
                      />

                      <Textarea
                        label="Легенда"
                        value={statement.legend}
                        onChange={(e) =>
                          patchStatement({ legend: e.currentTarget.value })
                        }
                        minRows={6}
                        maxRows={20}
                        autosize
                      />

                      <Textarea
                        label="Формат входных данных"
                        value={statement.input_format}
                        onChange={(e) =>
                          patchStatement({
                            input_format: e.currentTarget.value,
                          })
                        }
                        minRows={4}
                        maxRows={16}
                        autosize
                      />

                      <Textarea
                        label="Формат выходных данных"
                        value={statement.output_format}
                        onChange={(e) =>
                          patchStatement({
                            output_format: e.currentTarget.value,
                          })
                        }
                        minRows={4}
                        maxRows={16}
                        autosize
                      />

                      <Textarea
                        label="Примечания"
                        value={statement.notes}
                        onChange={(e) =>
                          patchStatement({ notes: e.currentTarget.value })
                        }
                        minRows={3}
                        maxRows={14}
                        autosize
                      />

                      <Textarea
                        label="Интерактивное взаимодействие"
                        value={statement.interaction}
                        onChange={(e) =>
                          patchStatement({ interaction: e.currentTarget.value })
                        }
                        minRows={3}
                        maxRows={14}
                        autosize
                      />

                      <Textarea
                        label="Система оценки"
                        value={statement.scoring}
                        onChange={(e) =>
                          patchStatement({ scoring: e.currentTarget.value })
                        }
                        minRows={3}
                        maxRows={14}
                        autosize
                      />

                      <Group justify="flex-end">
                        <Button
                          size="sm"
                          disabled={!isDirty}
                          loading={isSaving}
                          onClick={handleSave}
                        >
                          Сохранить условие
                        </Button>
                      </Group>
                    </>
                  )}
                </Stack>
              )}
            </SectionPaper>
          </Stack>
        </ScrollArea>
      </Box>

      <Box className={classes.previewPane} visibleFrom="md">
        <ScrollArea style={{ height: "100%" }} p="lg">
          <Stack gap="lg" maw={900} mx="auto">
            {isLoading || !deferredStatement || !hasPreviewMeta(previewMeta) ? (
              <Text c="dimmed" size="sm">
                Загрузка...
              </Text>
            ) : (
              <WorkshopStatementPreview
                statement={deferredStatement}
                previewMeta={previewMeta}
              />
            )}
          </Stack>
        </ScrollArea>
      </Box>
    </Box>
  );
}
