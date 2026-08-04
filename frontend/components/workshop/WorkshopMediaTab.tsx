"use client";

import { useState, useRef, useTransition } from "react";
import useSWR from "swr";
import {
  listWorkshopMediaFiles,
  uploadWorkshopMediaBinary,
  deleteWorkshopMediaFile,
} from "@/lib/actions";
import type { WorkshopFileTabProps } from "./WorkshopFileTabProps";
import {
  ActionIcon,
  Box,
  Button,
  Center,
  Code,
  CopyButton,
  Group,
  Loader,
  Modal,
  Paper,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import {
  IconCheck,
  IconCopy,
  IconPhoto,
  IconRefresh,
  IconTrash,
  IconUpload,
} from "@tabler/icons-react";
import classes from "./WorkshopFolderTab.module.css";

export function WorkshopMediaTab({
  problemId,
  selectedFile,
  onFileSelect,
  onFileCreated,
}: WorkshopFileTabProps) {
  const [isUploading, startUploading] = useTransition();
  const [isDeleting, startDeleting] = useTransition();
  const [deleteModalOpen, setDeleteModalOpen] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  const {
    data: filesData,
    isLoading,
    mutate,
  } = useSWR(["workshop-files", problemId, "media"], async () => {
    const [err, res] = await listWorkshopMediaFiles(problemId);
    if (err) throw new Error(err.message || "Не удалось загрузить список медиафайлов");
    return res;
  });

  const files = (filesData?.files || []).filter(
    (file) => !file.is_directory && file.path !== "tests.json"
  );

  const getFileName = (path: string) => path.split("/").pop() ?? path;

  const currentFileName = selectedFile ? getFileName(selectedFile) : null;
  const currentFileEntry = files.find((f) => f.path === selectedFile);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const uploadedFiles = e.target.files;
    if (!uploadedFiles || uploadedFiles.length === 0) return;

    const file = uploadedFiles[0];
    const formData = new FormData();
    formData.append("problemId", problemId);
    formData.append("name", file.name);
    formData.append("file", file);

    startUploading(async () => {
      const [err] = await uploadWorkshopMediaBinary(formData);
      if (err) {
        notifications.show({
          title: "Ошибка загрузки",
          message: err.message ?? "Не удалось загрузить файл",
          color: "red",
        });
        return;
      }

      notifications.show({
        title: "Файл загружен",
        message: file.name,
        color: "green",
      });

      const fullPath = `media/${file.name}`;
      mutate();
      onFileCreated(fullPath);
      if (fileInputRef.current) {
        fileInputRef.current.value = "";
      }
    });
  };

  const handleDelete = () => {
    if (!currentFileName) return;

    startDeleting(async () => {
      const [err] = await deleteWorkshopMediaFile(problemId, currentFileName);
      if (err) {
        notifications.show({
          title: "Ошибка удаления",
          message: err.message ?? "Не удалось удалить файл",
          color: "red",
        });
        return;
      }

      notifications.show({
        title: "Файл удалён",
        message: currentFileName,
        color: "green",
      });

      setDeleteModalOpen(false);
      mutate();
      onFileSelect("");
    });
  };

  const formatFileSize = (bytes?: number) => {
    if (!bytes) return null;
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  };

  const mediaUrl = currentFileName
    ? `/api/problems/${problemId}/media/${currentFileName}`
    : "";
  const markdownSnippet = currentFileName
    ? `![${currentFileName}](media/${currentFileName})`
    : "";

  return (
    <div className={classes.container}>
      <input
        type="file"
        ref={fileInputRef}
        onChange={handleFileChange}
        style={{ display: "none" }}
        accept="image/*,.pdf,.svg"
      />

      {/* Confirmation Modal for Deleting */}
      <Modal
        opened={deleteModalOpen}
        onClose={() => setDeleteModalOpen(false)}
        title="Удалить файл"
        centered
        size="sm"
      >
        <Stack gap="md">
          <Text size="sm">
            Вы уверены, что хотите удалить файл <Code>{currentFileName}</Code>? Это действие нельзя отменить.
          </Text>
          <Group justify="flex-end" gap="xs">
            <Button
              variant="subtle"
              color="gray"
              onClick={() => setDeleteModalOpen(false)}
              disabled={isDeleting}
            >
              Отмена
            </Button>
            <Button
              color="red"
              loading={isDeleting}
              onClick={handleDelete}
              leftSection={<IconTrash size={16} />}
            >
              Удалить
            </Button>
          </Group>
        </Stack>
      </Modal>

      {/* Left Sidebar - Files list & Upload button */}
      <div className={classes.sidebar}>
        <Button
          size="sm"
          variant="light"
          leftSection={<IconUpload size={16} />}
          onClick={() => fileInputRef.current?.click()}
          loading={isUploading}
          fullWidth
        >
          Загрузить изображение
        </Button>

        <div className={classes.fileList}>
          {isLoading ? (
            <Center py="xl">
              <Loader size="sm" />
            </Center>
          ) : files.length === 0 ? (
            <div className={classes.sidebarEmptyText}>нет файлов</div>
          ) : (
            files.map((file) => {
              const isActive = selectedFile === file.path;
              const fileName = getFileName(file.path!);

              return (
                <button
                  key={file.path}
                  type="button"
                  className={
                    isActive
                      ? `${classes.fileItem} ${classes.fileItemActive}`
                      : classes.fileItem
                  }
                  onClick={() => {
                    if (selectedFile !== file.path) onFileSelect(file.path!);
                  }}
                >
                  <IconPhoto size={16} style={{ flexShrink: 0 }} />
                  <span style={{ overflow: "hidden", textOverflow: "ellipsis" }}>
                    {fileName}
                  </span>
                </button>
              );
            })
          )}
        </div>
      </div>

      {/* Main View Area */}
      <div className={classes.editorArea}>
        <div className={classes.editorHeader}>
          <Group gap="xs">
            {selectedFile ? (
              <Code style={{ fontSize: 13 }}>{selectedFile}</Code>
            ) : (
              <Text size="sm" c="dimmed">
                Выберите файл из списка слева
              </Text>
            )}
          </Group>

          {selectedFile && (
            <Group gap="xs">
              <ActionIcon
                variant="subtle"
                title="Обновить"
                onClick={() => mutate()}
              >
                <IconRefresh size={16} />
              </ActionIcon>
              <Button
                size="xs"
                color="red"
                variant="light"
                leftSection={<IconTrash size={14} />}
                onClick={() => setDeleteModalOpen(true)}
              >
                Удалить
              </Button>
            </Group>
          )}
        </div>

        <div className={classes.editorWrapper}>
          {selectedFile && currentFileName ? (
            <Stack gap="md" align="center" style={{ width: "100%", maxWidth: 800, margin: "0 auto" }}>
              {/* Image Preview Box */}
              <Paper
                withBorder
                p="md"
                radius="md"
                style={{
                  width: "100%",
                  display: "flex",
                  justifyContent: "center",
                  alignItems: "center",
                  background: "light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-8))",
                  minHeight: 250,
                  maxHeight: 450,
                  overflow: "hidden",
                }}
              >
                <img
                  src={mediaUrl}
                  alt={currentFileName}
                  style={{
                    maxWidth: "100%",
                    maxHeight: 400,
                    objectFit: "contain",
                    borderRadius: 4,
                  }}
                />
              </Paper>

              {/* Controls and Copy Buttons */}
              <Paper withBorder p="md" radius="md" style={{ width: "100%" }}>
                <Stack gap="sm">
                  <Group justify="space-between">
                    <div>
                      <Text fw={600} size="sm">{currentFileName}</Text>
                      <Text size="xs" c="dimmed">
                        {currentFileEntry?.size ? formatFileSize(currentFileEntry.size) : "Медиафайл"}
                      </Text>
                    </div>

                    <Group gap="xs">
                      {/* Copy Markdown Button */}
                      <CopyButton value={markdownSnippet} timeout={2000}>
                        {({ copied, copy }) => (
                          <Button
                            size="xs"
                            variant={copied ? "filled" : "light"}
                            color={copied ? "teal" : "blue"}
                            leftSection={copied ? <IconCheck size={14} /> : <IconCopy size={14} />}
                            onClick={() => {
                              copy();
                              notifications.show({
                                title: "Скопировано!",
                                message: "Markdown-вставка скопирована в буфер обмена",
                                color: "teal",
                              });
                            }}
                          >
                            {copied ? "Скопировано" : "Скопировать Markdown"}
                          </Button>
                        )}
                      </CopyButton>

                      {/* Copy Link Button */}
                      <CopyButton value={mediaUrl} timeout={2000}>
                        {({ copied, copy }) => (
                          <Button
                            size="xs"
                            variant="outline"
                            color={copied ? "teal" : "gray"}
                            leftSection={copied ? <IconCheck size={14} /> : <IconCopy size={14} />}
                            onClick={() => {
                              copy();
                              notifications.show({
                                title: "Скопировано!",
                                message: "Прямая ссылка на файл скопирована в буфер обмена",
                                color: "teal",
                              });
                            }}
                          >
                            {copied ? "Скопировано" : "Скопировать ссылку"}
                          </Button>
                        )}
                      </CopyButton>
                    </Group>
                  </Group>

                  {/* Markdown code preview block */}
                  <Box
                    p="xs"
                    style={{
                      borderRadius: 4,
                      background: "light-dark(var(--mantine-color-gray-1), var(--mantine-color-dark-6))",
                      fontFamily: "var(--mantine-font-family-monospace)",
                      fontSize: 12,
                      wordBreak: "break-all",
                    }}
                  >
                    {markdownSnippet}
                  </Box>
                </Stack>
              </Paper>
            </Stack>
          ) : (
            <div className={classes.emptyState}>
              <Stack align="center" gap="xs">
                <IconPhoto size={48} color="var(--mantine-color-dimmed)" />
                <Title order={5} c="dimmed">
                  Выберите файл для просмотра или загрузите новое изображение
                </Title>
                <Button
                  size="xs"
                  variant="outline"
                  leftSection={<IconUpload size={14} />}
                  onClick={() => fileInputRef.current?.click()}
                  loading={isUploading}
                >
                  Загрузить картинку
                </Button>
              </Stack>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
