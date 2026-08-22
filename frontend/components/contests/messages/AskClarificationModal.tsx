"use client";

import {
  Button,
  Group,
  Modal,
  Select,
  Stack,
  Textarea,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {useState, type ReactNode} from "react";

import {api} from "@/lib/api";

import type {ContestProblemListItemModel} from "@/contracts/core/v1";

interface AskClarificationModalProps {
  opened: boolean;
  onClose: () => void;
  orgLogin: string;
  contestLogin: string;
  problems?: ContestProblemListItemModel[];
  onSuccess?: () => void;
}

export const AskClarificationModal = ({
  opened,
  onClose,
  orgLogin,
  contestLogin,
  problems = [],
  onSuccess,
}: AskClarificationModalProps): ReactNode => {
  const [problemId, setProblemId] = useState<string | null>(null);
  const [question, setQuestion] = useState("");
  const [loading, setLoading] = useState(false);

  const problemOptions = [
    {value: "", label: "Общий вопрос (не по конкретной задаче)"},
    ...problems.map((p) => {
      const letter = String.fromCharCode(65 + (p.position ?? 0));
      return {
        value: p.problem_id,
        label: `${letter}. ${p.title || "Задача"}`,
      };
    }),
  ];

  const handleSubmit = async () => {
    if (!question.trim()) {
      notifications.show({
        title: "Ошибка",
        message: "Введите текст вопроса",
        color: "red",
      });
      return;
    }

    setLoading(true);
    const [err] = await api.createContestClarification({
      orgLogin,
      contestLogin,
      requestBody: {
        question: question.trim(),
        problem_id: problemId ? problemId : undefined,
      },
    });
    setLoading(false);

    if (err) {
      notifications.show({
        title: "Ошибка отправки",
        message: err.message || "Не удалось отправить вопрос",
        color: "red",
      });
      return;
    }

    notifications.show({
      title: "Вопрос отправлен",
      message: "Жюри рассмотрит ваш вопрос и ответит в ближайшее время",
      color: "green",
    });

    setProblemId(null);
    setQuestion("");
    onClose();
    onSuccess?.();
  };

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Задать вопрос жюри"
      size="md"
    >
      <Stack gap="md">
        {problems.length > 0 && (
          <Select
            label="Задача (опционально)"
            placeholder="Выберите задачу"
            data={problemOptions}
            value={problemId || ""}
            onChange={(val) => setProblemId(val || null)}
            clearable
          />
        )}

        <Textarea
          label="Ваш вопрос"
          placeholder="Сформулируйте ваш вопрос четко и лаконично..."
          minRows={4}
          autosize
          maxRows={10}
          value={question}
          onChange={(e) => setQuestion(e.currentTarget.value)}
          required
        />

        <Group justify="flex-end" mt="sm">
          <Button variant="default" onClick={onClose} disabled={loading}>
            Отмена
          </Button>
          <Button onClick={handleSubmit} loading={loading}>
            Отправить вопрос
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
};
