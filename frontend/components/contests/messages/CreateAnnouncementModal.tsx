"use client";

import {
  Button,
  Group,
  Modal,
  Select,
  SegmentedControl,
  Stack,
  TextInput,
  Textarea,
  Paper,
  Text,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {useState, type ReactNode} from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import {api} from "@/lib/api";

import type {ContestProblemListItemModel} from "@/contracts/core/v1";

interface CreateAnnouncementModalProps {
  opened: boolean;
  onClose: () => void;
  orgLogin: string;
  contestLogin: string;
  problems?: ContestProblemListItemModel[];
  onSuccess?: () => void;
}

export const CreateAnnouncementModal = ({
  opened,
  onClose,
  orgLogin,
  contestLogin,
  problems = [],
  onSuccess,
}: CreateAnnouncementModalProps): ReactNode => {
  const [title, setTitle] = useState("");
  const [problemId, setProblemId] = useState<string | null>(null);
  const [body, setBody] = useState("");
  const [tab, setTab] = useState<"edit" | "preview">("edit");
  const [loading, setLoading] = useState(false);

  const problemOptions = [
    {value: "", label: "Общее объявление (без привязки к задаче)"},
    ...problems.map((p) => {
      const letter = String.fromCharCode(65 + (p.position ?? 0));
      return {
        value: p.problem_id,
        label: `${letter}. ${p.title || "Задача"}`,
      };
    }),
  ];

  const handleSubmit = async () => {
    if (!title.trim()) {
      notifications.show({
        title: "Ошибка",
        message: "Укажите заголовок объявления",
        color: "red",
      });
      return;
    }
    if (!body.trim()) {
      notifications.show({
        title: "Ошибка",
        message: "Укажите текст объявления",
        color: "red",
      });
      return;
    }

    setLoading(true);
    const [err] = await api.createContestAnnouncement({
      orgLogin,
      contestLogin,
      requestBody: {
        title: title.trim(),
        body: body.trim(),
        problem_id: problemId ? problemId : undefined,
      },
    });
    setLoading(false);

    if (err) {
      notifications.show({
        title: "Ошибка публикации",
        message: err.message || "Не удалось создать объявление",
        color: "red",
      });
      return;
    }

    notifications.show({
      title: "Успешно",
      message: "Объявление опубликовано",
      color: "green",
    });

    setTitle("");
    setProblemId(null);
    setBody("");
    setTab("edit");
    onClose();
    onSuccess?.();
  };

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Новое объявление жюри"
      size="lg"
    >
      <Stack gap="md">
        <TextInput
          label="Заголовок объявления"
          placeholder="например, Исправление в условии задачи A"
          required
          value={title}
          onChange={(e) => setTitle(e.currentTarget.value)}
        />

        {problems.length > 0 && (
          <Select
            label="Привязка к задаче (опционально)"
            placeholder="Выберите задачу"
            data={problemOptions}
            value={problemId || ""}
            onChange={(val) => setProblemId(val || null)}
            clearable
          />
        )}

        <Group justify="space-between" align="center">
          <Text size="sm" fw={500}>
            Текст объявления (Markdown)
          </Text>
          <SegmentedControl
            size="xs"
            value={tab}
            onChange={(val) => setTab(val as "edit" | "preview")}
            data={[
              {label: "Редактор", value: "edit"},
              {label: "Предпросмотр", value: "preview"},
            ]}
          />
        </Group>

        {tab === "edit" ? (
          <Textarea
            placeholder="Введите текст объявления... Поддерживается разметка Markdown."
            minRows={5}
            autosize
            maxRows={15}
            value={body}
            onChange={(e) => setBody(e.currentTarget.value)}
            required
          />
        ) : (
          <Paper withBorder p="sm" mih={120} bg="var(--mantine-color-default-hover)">
            {body.trim() ? (
              <ReactMarkdown remarkPlugins={[remarkGfm]}>{body}</ReactMarkdown>
            ) : (
              <Text c="dimmed" size="sm">
                (Текст объявления пуст)
              </Text>
            )}
          </Paper>
        )}

        <Group justify="flex-end" mt="sm">
          <Button variant="default" onClick={onClose} disabled={loading}>
            Отмена
          </Button>
          <Button onClick={handleSubmit} loading={loading}>
            Опубликовать
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
};
