"use client";

import {
  Alert,
  Box,
  Button,
  Group,
  LoadingOverlay,
  Modal,
  Stack,
  Text,
  Textarea,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useEffect, useState, useTransition } from "react";

import { getWorkshopTestFile, updateWorkshopTestFile } from "@/lib/actions";

type Props = {
  opened: boolean;
  onClose: () => void;
  problemId: string;
  filename: string | null;
  onSaved?: () => void;
};

const MAX_PREVIEW_BYTES = 50 * 1024; // 50 KB

export const TestPreviewModal = ({
  opened,
  onClose,
  problemId,
  filename,
  onSaved,
}: Props) => {
  const [content, setContent] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isSaving, startSaving] = useTransition();
  const [isTruncated, setIsTruncated] = useState(false);
  const [originalError, setOriginalError] = useState<string | null>(null);

  useEffect(() => {
    if (opened && filename) {
      setIsLoading(true);
      setOriginalError(null);
      getWorkshopTestFile(problemId, filename).then(([err, text]) => {
        setIsLoading(false);
        if (err || text === null) {
          setOriginalError(err?.message || "Файл отсутствует или пуст");
          setContent("");
        } else {
          if (text.length > MAX_PREVIEW_BYTES) {
            setContent(text.slice(0, MAX_PREVIEW_BYTES));
            setIsTruncated(true);
          } else {
            setContent(text);
            setIsTruncated(false);
          }
        }
      });
    }
  }, [opened, filename, problemId]);

  const handleSave = () => {
    if (!filename) {
      return;
    }

    startSaving(async () => {
      const [err] = await updateWorkshopTestFile(problemId, filename, content);
      if (err) {
        notifications.show({
          title: "Ошибка сохранения",
          message: err.message || "Не удалось сохранить файл",
          color: "red",
        });
        return;
      }

      notifications.show({
        title: "Сохранено",
        message: `Файл ${filename} успешно обновлен`,
        color: "green",
      });
      onSaved?.();
      onClose();
    });
  };

  const handleDownload = () => {
    if (!filename) {
      return;
    }
    const blob = new Blob([content], { type: "text/plain;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const a = document.createElement("a");
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);
  };

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title={
        <Text fw={600} size="md">
          Просмотр файла: {filename}
        </Text>
      }
      size="lg"
    >
      <Box pos="relative" mih={200}>
        <LoadingOverlay visible={isLoading} />

        <Stack gap="md">
          {originalError && (
            <Alert color="red" title="Ошибка загрузки">
              {originalError}
            </Alert>
          )}

          {isTruncated && (
            <Alert color="yellow" title="Большой размер файла">
              Отображаются первые 50 КБ файла. Редактирование большого файла сбросит
              данные после превью. Воспользуйтесь кнопкой скачивания оригинала.
            </Alert>
          )}

          <Textarea
            label="Содержимое файла"
            autosize
            minRows={10}
            maxRows={20}
            value={content}
            onChange={(e) => setContent(e.currentTarget.value)}
            style={{ fontFamily: "monospace" }}
          />

          <Group justify="space-between">
            <Button variant="default" onClick={handleDownload}>
              Скачать файл
            </Button>

            <Group>
              <Button variant="subtle" color="gray" onClick={onClose}>
                Отмена
              </Button>
              <Button loading={isSaving} onClick={handleSave}>
                Сохранить
              </Button>
            </Group>
          </Group>
        </Stack>
      </Box>
    </Modal>
  );
};
