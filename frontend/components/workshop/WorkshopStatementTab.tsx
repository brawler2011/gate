"use client";

import {
  Button,
  Group,
  ScrollArea,
  Stack,
  Text,
  TextInput,
  Textarea,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useEffect, useState, useTransition } from "react";
import { SectionPaper } from "@/components/workshop/SectionPaper";
import { getWorkshopFile, saveWorkshopFile } from "@/lib/actions";

type StatementData = {
  title: string;
  legend: string;
  input_format: string;
  output_format: string;
  notes: string;
  interaction: string;
  scoring: string;
};

type Props = {
  problemId: string;
};

const EMPTY_STATEMENT: StatementData = {
  title: "",
  legend: "",
  input_format: "",
  output_format: "",
  notes: "",
  interaction: "",
  scoring: "",
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function toStatementData(value: unknown): StatementData {
  if (!isRecord(value)) return EMPTY_STATEMENT;

  return {
    title: typeof value.title === "string" ? value.title : "",
    legend: typeof value.legend === "string" ? value.legend : "",
    input_format: typeof value.input_format === "string" ? value.input_format : "",
    output_format: typeof value.output_format === "string" ? value.output_format : "",
    notes: typeof value.notes === "string" ? value.notes : "",
    interaction: typeof value.interaction === "string" ? value.interaction : "",
    scoring: typeof value.scoring === "string" ? value.scoring : "",
  };
}

export function WorkshopStatementTab({ problemId }: Props) {
  const [statement, setStatement] = useState<StatementData>(EMPTY_STATEMENT);
  const [isDirty, setIsDirty] = useState(false);
  const [isLoading, startLoading] = useTransition();
  const [isSaving, startSaving] = useTransition();

  useEffect(() => {
    startLoading(async () => {
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
        const parsed = JSON.parse(typeof data === "string" ? data : "{}");
        const statementValue = isRecord(parsed) ? parsed.statement : undefined;
        setStatement(toStatementData(statementValue));
        setIsDirty(false);
      } catch {
        notifications.show({
          title: "Ошибка парсинга manifest.json",
          message: "Файл содержит невалидный JSON",
          color: "red",
        });
      }
    });
  }, [problemId]);

  const patchStatement = (patch: Partial<StatementData>) => {
    setStatement((prev) => ({ ...prev, ...patch }));
    setIsDirty(true);
  };

  const handleSave = () => {
    startSaving(async () => {
      const [currentError, currentData] = await getWorkshopFile(problemId, "manifest.json");
      if (currentError) {
        notifications.show({
          title: "Ошибка сохранения",
          message: currentError.message ?? "Не удалось загрузить актуальный manifest.json",
          color: "red",
        });
        return;
      }

      let manifest: Record<string, unknown> = {};
      try {
        const parsed = JSON.parse(typeof currentData === "string" ? currentData : "{}");
        if (isRecord(parsed)) {
          manifest = parsed;
        }
      } catch {
        notifications.show({
          title: "Ошибка сохранения",
          message: "manifest.json содержит невалидный JSON",
          color: "red",
        });
        return;
      }

      const currentStatement = isRecord(manifest.statement) ? manifest.statement : {};

      manifest.statement = {
        ...currentStatement,
        ...statement,
      };

      const [saveError] = await saveWorkshopFile(
        problemId,
        "manifest.json",
        JSON.stringify(manifest, null, 2)
      );

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
        message: "Условие записано в manifest.json",
        color: "green",
      });
    });
  };

  return (
    <ScrollArea style={{ flex: 1 }} p="lg">
      <Stack gap="lg" maw={900} mx="auto">
        <SectionPaper title="Условие задачи (statement)">
          {isLoading ? (
            <Text c="dimmed" size="sm">
              Загрузка...
            </Text>
          ) : (
            <Stack gap="md">
              <TextInput
                label="Заголовок"
                value={statement.title}
                onChange={(e) => patchStatement({ title: e.currentTarget.value })}
              />

              <Textarea
                label="Легенда"
                value={statement.legend}
                onChange={(e) => patchStatement({ legend: e.currentTarget.value })}
                minRows={6}
                maxRows={20}
                autosize
              />

              <Textarea
                label="Формат входных данных"
                value={statement.input_format}
                onChange={(e) => patchStatement({ input_format: e.currentTarget.value })}
                minRows={4}
                maxRows={16}
                autosize
              />

              <Textarea
                label="Формат выходных данных"
                value={statement.output_format}
                onChange={(e) => patchStatement({ output_format: e.currentTarget.value })}
                minRows={4}
                maxRows={16}
                autosize
              />

              <Textarea
                label="Примечания"
                value={statement.notes}
                onChange={(e) => patchStatement({ notes: e.currentTarget.value })}
                minRows={3}
                maxRows={14}
                autosize
              />

              <Textarea
                label="Интерактивное взаимодействие"
                value={statement.interaction}
                onChange={(e) => patchStatement({ interaction: e.currentTarget.value })}
                minRows={3}
                maxRows={14}
                autosize
              />

              <Textarea
                label="Система оценки"
                value={statement.scoring}
                onChange={(e) => patchStatement({ scoring: e.currentTarget.value })}
                minRows={3}
                maxRows={14}
                autosize
              />

              <Group justify="flex-end">
                <Button size="sm" disabled={!isDirty} loading={isSaving} onClick={handleSave}>
                  Сохранить условие
                </Button>
              </Group>
            </Stack>
          )}
        </SectionPaper>
      </Stack>
    </ScrollArea>
  );
}
