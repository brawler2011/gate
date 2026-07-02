"use client";

import type { ApiError } from "@/lib/api";
import type { ProblemModel } from "@contracts/core/v1";
import {
  Button,
  Container,
  Divider,
  FileInput,
  Group,
  Modal,
  NumberInput,
  Paper,
  Stack,
  Textarea,
  TextInput,
  Title,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import { IconArrowLeft, IconUpload } from "@tabler/icons-react";
import Link from "next/link";
import useSWRMutation from "swr/mutation";
import { useRouter } from "next/navigation";
import React, { useState } from "react";

type ProblemFormData = {
  title?: string;
  time_limit?: number;
  memory_limit?: number;
  legend?: string;
  input_format?: string;
  output_format?: string;
  notes?: string;
  scoring?: string;
};

type Props = {
  problem: ProblemModel;
  onSubmitFn: (
    id: string,
    data: ProblemFormData,
  ) => Promise<readonly [ApiError | null, unknown]>;
  onUploadFn: (
    id: string,
    data: FormData,
  ) => Promise<readonly [ApiError | null, unknown]>;
};

const ProblemForm = ({ problem, onSubmitFn, onUploadFn }: Props) => {
  const router = useRouter();
  const [opened, setOpened] = useState(false);
  const [file, setFile] = useState<File | null>(null);

  const form = useForm({
    initialValues: {
      title: problem.title || "",
      time_limit: problem.time_limit || 1000,
      memory_limit: problem.memory_limit || 256,
      legend: problem.legend || "",
      input_format: problem.input_format || "",
      output_format: problem.output_format || "",
      notes: problem.notes || "",
      scoring: problem.scoring || "",
    },
  });

  const { trigger: triggerUpdate, isMutating: isUpdating } = useSWRMutation(
    `problem/update/${problem.id}`,
    async (_, { arg }: { arg: ProblemFormData }) => {
      const [error, response] = await onSubmitFn(problem.id, arg);
      if (error) throw new Error(error.message);
      return response;
    },
    {
      onSuccess: async () => {
        console.log("✅ Problem updated successfully");
        form.resetDirty();
        router.refresh();
      },
      onError: (error) => {
        console.error("❌ Problem update failed:", error);
        notifications.show({
          title: "Ошибка",
          message:
            error instanceof Error ? error.message : "Не удалось обновить задачу",
          color: "red",
        });
        form.clearErrors();
      },
    }
  );

  const { trigger: triggerUpload, isMutating: isUploading } = useSWRMutation(
    `problem/upload/${problem.id}`,
    async (_, { arg }: { arg: FormData }) => {
      const [error, response] = await onUploadFn(problem.id, arg);
      if (error) throw new Error(error.message);
      return response;
    },
    {
      onSuccess: () => {
        setOpened(false);
        setFile(null);
        notifications.show({
          title: "Успешно",
          message: "Файл загружен",
          color: "green",
        });
      },
      onError: (error) => {
        console.error("Upload failed:", error);
        notifications.show({
          title: "Ошибка",
          message:
            error instanceof Error ? error.message : "Не удалось загрузить файл",
          color: "red",
        });
      },
    }
  );

  const onSubmit = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const values = form.getValues();
    console.log("📝 ProblemForm - submitting values:", values);
    triggerUpdate(values);
  };

  const handleUpload = () => {
    if (file) {
      const formData = new FormData();
      formData.append("file", file);
      triggerUpload(formData);
    }
  };

  return (
    <>
      <Paper shadow="sm" p="md" mb="lg" withBorder>
        <Group justify="space-between">
          <Group gap="sm">
            <Link
              href={
                problem.organization_id
                  ? `/orgs/${problem.organization_id}/problems`
                  : "/orgs"
              }
              style={{ textDecoration: "none" }}
            >
              <Button
                variant="default"
                size="sm"
                leftSection={<IconArrowLeft size={16} />}
              >
                Назад
              </Button>
            </Link>
            <div>
              <Title order={3} size="h5">
                Редактирование задачи
              </Title>
            </div>
          </Group>
          <Group gap="sm">
            <Button
              variant="default"
              size="sm"
              leftSection={<IconUpload size={16} />}
              onClick={() => setOpened(true)}
            >
              Загрузить файл
            </Button>
            <Button
              type="submit"
              form="problem-form"
              size="sm"
              disabled={!form.isDirty()}
              loading={isUpdating}
            >
              Сохранить
            </Button>
          </Group>
        </Group>
      </Paper>

      <Container size="md" pt={0} pb="xl">
        <form id="problem-form" onSubmit={onSubmit}>
          <Stack gap="lg">
            {/* Title Section */}
            <Paper p="lg" radius="md" withBorder>
              <Stack gap="md">
                <div>
                  <Title order={4} mb="xs">
                    📝 Основная информация
                  </Title>
                  <Divider />
                </div>
                <TextInput
                  label="Название задачи"
                  placeholder="Введите название"
                  size="md"
                  key={form.key("title")}
                  {...form.getInputProps("title")}
                />
              </Stack>
            </Paper>

            {/* Limits Section */}
            <Paper p="lg" radius="md" withBorder>
              <Stack gap="md">
                <div>
                  <Title order={4} mb="xs">
                    ⚙️ Ограничения
                  </Title>
                  <Divider />
                </div>
                <Group grow>
                  <NumberInput
                    label="Время (мс)"
                    placeholder="1000"
                    size="md"
                    key={form.key("time_limit")}
                    {...form.getInputProps("time_limit")}
                  />
                  <NumberInput
                    label="Память (МБ)"
                    placeholder="64"
                    size="md"
                    key={form.key("memory_limit")}
                    {...form.getInputProps("memory_limit")}
                  />
                </Group>
              </Stack>
            </Paper>

            {/* Description Section */}
            <Paper p="lg" radius="md" withBorder>
              <Stack gap="md">
                <div>
                  <Title order={4} mb="xs">
                    📄 Условие задачи
                  </Title>
                  <Divider />
                </div>
                <Textarea
                  label="Легенда"
                  placeholder="Опишите условие задачи..."
                  autosize
                  minRows={4}
                  maxRows={8}
                  size="md"
                  key={form.key("legend")}
                  {...form.getInputProps("legend")}
                />
                <Textarea
                  label="Формат входных данных"
                  placeholder="Опишите входные данные..."
                  autosize
                  minRows={3}
                  maxRows={6}
                  size="md"
                  key={form.key("input_format")}
                  {...form.getInputProps("input_format")}
                />
                <Textarea
                  label="Формат выходных данных"
                  placeholder="Опишите выходные данные..."
                  autosize
                  minRows={3}
                  maxRows={6}
                  size="md"
                  key={form.key("output_format")}
                  {...form.getInputProps("output_format")}
                />
              </Stack>
            </Paper>

            {/* Scoring and Notes Section */}
            <Paper p="lg" radius="md" withBorder>
              <Stack gap="md">
                <div>
                  <Title order={4} mb="xs">
                    📊 Дополнительные параметры
                  </Title>
                  <Divider />
                </div>
                <Textarea
                  label="Система оценки"
                  placeholder="Опишите систему оценки..."
                  autosize
                  minRows={3}
                  maxRows={6}
                  size="md"
                  key={form.key("scoring")}
                  {...form.getInputProps("scoring")}
                />
                <Textarea
                  label="Примечание"
                  placeholder="Дополнительные примечания..."
                  autosize
                  minRows={2}
                  maxRows={4}
                  size="md"
                  key={form.key("notes")}
                  {...form.getInputProps("notes")}
                />
              </Stack>
            </Paper>

            {/* Save Button */}
            <Group justify="flex-end" gap="md">
              <Button variant="default" size="md" onClick={() => router.back()}>
                Отмена
              </Button>
              <Button
                type="submit"
                size="md"
                loading={isUpdating}
                disabled={!form.isDirty()}
              >
                Сохранить изменения
              </Button>
            </Group>
          </Stack>
        </form>
      </Container>

      {/* Modal for file upload */}
      <Modal
        opened={opened}
        onClose={() => setOpened(false)}
        title="Загрузить файл"
        centered
      >
        <Stack>
          <FileInput
            label="Выберите файл"
            placeholder="Выберите файл"
            onChange={setFile}
            value={file}
          />
          <Button
            onClick={handleUpload}
            disabled={!file}
            loading={isUploading}
          >
            Загрузить
          </Button>
        </Stack>
      </Modal>
    </>
  );
};

export { ProblemForm };
