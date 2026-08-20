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
import {notifications} from "@mantine/notifications";
import {useEffect, useState, useTransition} from "react";

import {api} from "@/lib/api";

import type {ReactNode} from "react";

type Props = {
  opened: boolean;
  onClose: () => void;
  problemId: string;
  filename: string | null;
  fileSize?: number;
  onSaved?: () => void;
};

const MAX_PREVIEW_BYTES = 100 * 1024; // 100 KB

const formatBytes = (bytes?: number): string => {
  if (bytes === undefined || bytes === null) {
    return "";
  }
  if (bytes < 1024) {
    return `${bytes} Б`;
  }
  if (bytes < 1024 * 1024) {
    return `${(bytes / 1024).toFixed(1)} КБ`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1)} МБ`;
};

export const TestPreviewModal = ({
  opened,
  onClose,
  problemId,
  filename,
  fileSize,
  onSaved,
}: Props): ReactNode => {
  const [content, setContent] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isDownloading, setIsDownloading] = useState(false);
  const [isSaving, startSaving] = useTransition();
  const [isTruncated, setIsTruncated] = useState(false);
  const [isTooLarge, setIsTooLarge] = useState(false);
  const [originalError, setOriginalError] = useState<string | null>(null);

  useEffect(() => {
    if (opened && filename) {
      setOriginalError(null);
      setContent("");
      setIsTruncated(false);

      if (typeof fileSize === "number" && fileSize > MAX_PREVIEW_BYTES) {
        setIsTooLarge(true);
        setIsLoading(false);
      } else {
        setIsTooLarge(false);
        setIsLoading(true);
        api.getProblemTestFile({problemId, name: filename}).then(async ([err, blob]) => {
          setIsLoading(false);
          if (err || !blob) {
            setOriginalError(err?.message || "Файл отсутствует или пуст");
            setContent("");
          } else {
            const text = await blob.text();
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
    }
  }, [opened, filename, fileSize, problemId]);

  const handleLoadPartial = async () => {
    if (!filename) {
      return;
    }
    setIsLoading(true);
    const [err, blob] = await api.getProblemTestFile({problemId, name: filename});
    setIsLoading(false);
    if (err || !blob) {
      setOriginalError(err?.message || "Ошибка загрузки файла");
      return;
    }
    const sliced = blob.slice(0, MAX_PREVIEW_BYTES);
    const text = await sliced.text();
    setContent(text);
    setIsTruncated(true);
    setIsTooLarge(false);
  };

  const handleSave = () => {
    if (!filename) {
      return;
    }

    startSaving(async () => {
      const blob = new Blob([content], {type: "application/octet-stream"});
      const [err] = await api.updateProblemTestFile({problemId, name: filename, requestBody: blob});
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

  const handleDownload = async () => {
    if (!filename) {
      return;
    }
    setIsDownloading(true);
    const [err, blob] = await api.getProblemTestFile({problemId, name: filename});
    setIsDownloading(false);
    if (err || !blob) {
      notifications.show({
        title: "Ошибка скачивания",
        message: err?.message || "Не удалось скачать файл",
        color: "red",
      });
      return;
    }
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
          Просмотр файла: {filename} {fileSize !== undefined ? `(${formatBytes(fileSize)})` : ""}
        </Text>
      }
      size="lg"
    >
      <Box pos="relative" mih={200}>
        <LoadingOverlay visible={isLoading || isDownloading} />

        <Stack gap="md">
          {originalError && (
            <Alert color="red" title="Ошибка загрузки">
              {originalError}
            </Alert>
          )}

          {isTooLarge && (
            <Alert color="yellow" title="Большой размер файла">
              <Stack gap="xs">
                <Text size="sm">
                  Размер файла составляет {formatBytes(fileSize)} (больше 100 КБ).
                  Автоматическая загрузка отключена во избежание зависания браузера.
                </Text>
                <Group gap="sm" mt="xs">
                  <Button variant="default" size="xs" onClick={handleDownload} loading={isDownloading}>
                    Скачать файл ({formatBytes(fileSize)})
                  </Button>
                  <Button variant="outline" size="xs" onClick={handleLoadPartial} loading={isLoading}>
                    Показать первые 100 КБ
                  </Button>
                </Group>
              </Stack>
            </Alert>
          )}

          {isTruncated && (
            <Alert color="yellow" title="Предпросмотр ограничен">
              Отображаются первые 100 КБ файла. Редактирование и сохранение большого файла
              перезапишет файл только содержимым превью. Воспользуйтесь кнопкой скачивания
              оригинала для полного доступа.
            </Alert>
          )}

          {!isTooLarge && (
            <Textarea
              label="Содержимое файла"
              autosize
              minRows={10}
              maxRows={20}
              value={content}
              onChange={(e) => setContent(e.currentTarget.value)}
              style={{fontFamily: "monospace"}}
            />
          )}

          <Group justify="space-between">
            <Button variant="default" onClick={handleDownload} loading={isDownloading}>
              Скачать файл
            </Button>

            <Group>
              <Button variant="subtle" color="gray" onClick={onClose}>
                Отмена
              </Button>
              {!isTooLarge && (
                <Button loading={isSaving} onClick={handleSave}>
                  Сохранить
                </Button>
              )}
            </Group>
          </Group>
        </Stack>
      </Box>
    </Modal>
  );
};
