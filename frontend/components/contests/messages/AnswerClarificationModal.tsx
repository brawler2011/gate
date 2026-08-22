"use client";

import {
  Badge,
  Button,
  Checkbox,
  Group,
  Modal,
  Paper,
  Stack,
  Text,
  TextInput,
  Textarea,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {useEffect, useState, type ReactNode} from "react";

import {api} from "@/lib/api";

import type {ContestClarificationModel} from "@/contracts/core/v1";

interface AnswerClarificationModalProps {
  opened: boolean;
  onClose: () => void;
  orgLogin: string;
  contestLogin: string;
  clarification: ContestClarificationModel | null;
  onSuccess?: () => void;
}

const QUICK_TEMPLATES = [
  "Без комментариев",
  "Внимательно прочитайте условие",
  "Да",
  "Нет",
];

export const AnswerClarificationModal = ({
  opened,
  onClose,
  orgLogin,
  contestLogin,
  clarification,
  onSuccess,
}: AnswerClarificationModalProps): ReactNode => {
  const [answer, setAnswer] = useState("");
  const [publishAsAnnouncement, setPublishAsAnnouncement] = useState(false);
  const [announcementTitle, setAnnouncementTitle] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (clarification) {
      setAnswer(clarification.answer || "");
      setPublishAsAnnouncement(false);
      if (clarification.problem_letter) {
        setAnnouncementTitle(`Уточнение по задаче ${clarification.problem_letter}`);
      } else {
        setAnnouncementTitle("Ответ на вопрос участников");
      }
    }
  }, [clarification]);

  if (!clarification) {
    return null;
  }

  const handleSubmit = async () => {
    if (!answer.trim()) {
      notifications.show({
        title: "Ошибка",
        message: "Введите текст ответа",
        color: "red",
      });
      return;
    }

    setLoading(true);
    const [err] = await api.answerContestClarification({
      orgLogin,
      contestLogin,
      clarificationId: clarification.id,
      requestBody: {
        answer: answer.trim(),
        publish_as_announcement: publishAsAnnouncement,
        announcement_title: publishAsAnnouncement ? announcementTitle.trim() : undefined,
      },
    });
    setLoading(false);

    if (err) {
      notifications.show({
        title: "Ошибка",
        message: err.message || "Не удалось отправить ответ",
        color: "red",
      });
      return;
    }

    notifications.show({
      title: "Ответ отправлен",
      message: publishAsAnnouncement
        ? "Ответ сохранен и опубликован как объявление для всех"
        : "Ответ успешно отправлен участнику",
      color: "green",
    });

    onClose();
    onSuccess?.();
  };

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Ответ на вопрос участника"
      size="lg"
    >
      <Stack gap="md">
        <Paper withBorder p="sm" bg="var(--mantine-color-default-hover)" radius="sm">
          <Group justify="space-between" mb="xs">
            <Group gap="xs">
              <Text size="sm" fw={600}>
                {clarification.username || "Участник"}
              </Text>
              {clarification.problem_letter && (
                <Badge size="sm" variant="light">
                  Задача {clarification.problem_letter}
                  {clarification.problem_title ? ` (${clarification.problem_title})` : ""}
                </Badge>
              )}
            </Group>
          </Group>
          <Text size="sm" style={{whiteSpace: "pre-wrap"}}>
            {clarification.question}
          </Text>
        </Paper>

        <div>
          <Text size="xs" c="dimmed" mb={4} fw={500}>
            Быстрые шаблоны ответов:
          </Text>
          <Group gap="xs">
            {QUICK_TEMPLATES.map((tmpl) => (
              <Button
                key={tmpl}
                size="xs"
                variant="light"
                color="gray"
                onClick={() => setAnswer(tmpl)}
              >
                {tmpl}
              </Button>
            ))}
          </Group>
        </div>

        <Textarea
          label="Текст ответа"
          placeholder="Введите ответ жюри..."
          minRows={3}
          autosize
          maxRows={8}
          value={answer}
          onChange={(e) => setAnswer(e.currentTarget.value)}
          required
        />

        <Checkbox
          label="Опубликовать как объявление для всех участников"
          description="Вопрос и ответ станут видны всем участникам во вкладке Объявления"
          checked={publishAsAnnouncement}
          onChange={(e) => setPublishAsAnnouncement(e.currentTarget.checked)}
        />

        {publishAsAnnouncement && (
          <TextInput
            label="Заголовок объявления"
            placeholder="например, Уточнение по задаче A"
            value={announcementTitle}
            onChange={(e) => setAnnouncementTitle(e.currentTarget.value)}
          />
        )}

        <Group justify="flex-end" mt="sm">
          <Button variant="default" onClick={onClose} disabled={loading}>
            Отмена
          </Button>
          <Button onClick={handleSubmit} loading={loading} color="teal">
            Отправить ответ
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
};
