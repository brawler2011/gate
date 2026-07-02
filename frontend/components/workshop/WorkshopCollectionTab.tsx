"use client";

import type { ApiError } from "@/lib/api";
import type { FileEntry } from "@contracts/core/v1";
import {
  ActionIcon,
  Box,
  Button,
  Center,
  Code,
  Group,
  Loader,
  Stack,
  Text,
  TextInput,
  Textarea,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconFile, IconPlus, IconRefresh, IconX, IconStar } from "@tabler/icons-react";
import { useEffect, useRef, useState, useTransition } from "react";
import classes from "./WorkshopFolderTab.module.css";
import useSWR from "swr";

type ListFilesResult = Promise<
  [ApiError | null, { files?: FileEntry[] } | null]
>;
type GetFileResult = Promise<[ApiError | null, string | null]>;
type SaveFileResult = Promise<[ApiError | null, unknown | null]>;

type Props = {
  problemId: string;
  folderName: string;
  selectedFile: string | null;
  onFileSelect: (filePath: string) => void;
  onFileCreated: (filePath: string) => void;
  listFiles: (problemId: string) => ListFilesResult;
  getFile: (problemId: string, name: string) => GetFileResult;
  createFile: (
    problemId: string,
    name: string,
    content: string,
  ) => SaveFileResult;
  updateFile: (
    problemId: string,
    name: string,
    content: string,
  ) => SaveFileResult;
  setMain?: (
    problemId: string,
    name: string,
  ) => SaveFileResult;
};

export function WorkshopCollectionTab({
  problemId,
  folderName,
  selectedFile,
  onFileSelect,
  onFileCreated,
  listFiles,
  getFile,
  createFile,
  updateFile,
  setMain,
}: Props) {
  const [content, setContent] = useState<string>("");
  const [isDirty, setIsDirty] = useState(false);
  const [isSaving, startSaving] = useTransition();

  const [isCreating, setIsCreating] = useState(false);
  const [newFileName, setNewFileName] = useState("");
  const [isCreatingFile, startCreatingFile] = useTransition();
  const newFileInputRef = useRef<HTMLInputElement>(null);

  const getFileName = (path: string) => path.split("/").pop() ?? path;

  const { data: filesData, isLoading: isLoadingFiles, mutate: mutateFiles } = useSWR(
    ["workshop-files", problemId, folderName],
    async () => {
      const [err, res] = await listFiles(problemId);
      if (err) throw err;
      return res;
    }
  );

  const files = filesData?.files || [];

  const fileName = selectedFile ? getFileName(selectedFile) : null;
  const { data: fileContent, isLoading: isLoadingFile, mutate: mutateContent } = useSWR(
    fileName ? ["workshop-file-content", problemId, folderName, fileName] : null,
    async () => {
      const [err, res] = await getFile(problemId, fileName!);
      if (err) throw err;
      return res;
    }
  );

  const leafFiles = files.filter((file) => !file.is_directory);

  useEffect(() => {
    if (fileContent !== undefined && selectedFile) {
      setContent(fileContent || "");
      setIsDirty(false);
    }
  }, [fileContent, selectedFile]);

  useEffect(() => {
    if (!selectedFile) {
      setContent("");
      setIsDirty(false);
    }
  }, [selectedFile]);

  useEffect(() => {
    if (!selectedFile && leafFiles.length === 1) {
      onFileSelect(leafFiles[0].path!);
    }
  }, [leafFiles, onFileSelect, selectedFile]);

  const handleSave = () => {
    if (!selectedFile) return;

    startSaving(async () => {
      const fileName = getFileName(selectedFile);
      const isExistingFile = leafFiles.some(
        (file) => file.path === selectedFile,
      );
      const saveFile = isExistingFile ? updateFile : createFile;
      const [error] = await saveFile(problemId, fileName, content);

      if (error) {
        notifications.show({
          title: isExistingFile ? "Ошибка обновления" : "Ошибка создания",
          message: error.message ?? "Не удалось сохранить файл",
          color: "red",
        });
        return;
      }

      setIsDirty(false);
      notifications.show({
        title: isExistingFile ? "Обновлено" : "Создано",
        message: selectedFile,
        color: "green",
      });
      mutateFiles();
      mutateContent();
    });
  };

  const openCreateInput = () => {
    setIsCreating(true);
    setNewFileName("");
    setTimeout(() => newFileInputRef.current?.focus(), 0);
  };

  const cancelCreate = () => {
    setIsCreating(false);
    setNewFileName("");
  };

  const handleCreate = () => {
    const trimmed = newFileName.trim();
    if (!trimmed) return;

    const fullPath = `${folderName}/${trimmed}`;

    startCreatingFile(async () => {
      const [error] = await createFile(problemId, trimmed, "");
      if (error) {
        notifications.show({
          title: "Ошибка создания файла",
          message: error.message ?? "Не удалось создать файл",
          color: "red",
        });
        return;
      }

      setIsCreating(false);
      setNewFileName("");
      mutateFiles();
      onFileCreated(fullPath);
    });
  };

  const fileBar = (
    <div className={classes.fileBar}>
      {leafFiles.map((file) => (
        <button
          key={file.path}
          type="button"
          className={`${classes.filePill} ${
            selectedFile === file.path ? classes.filePillActive : ""
          }`}
          onClick={() => {
            if (selectedFile !== file.path) onFileSelect(file.path!);
          }}
        >
          <Group gap={4} wrap="nowrap" align="center" style={{ display: "inline-flex" }}>
            {file.is_main && <IconStar size={12} fill="currentColor" color="var(--mantine-color-yellow-5)" />}
            <span>{getFileName(file.path!)}</span>
          </Group>
        </button>
      ))}

      {isCreating ? (
        <Group gap={4} style={{ flexShrink: 0 }}>
          <TextInput
            ref={newFileInputRef}
            value={newFileName}
            onChange={(event) => setNewFileName(event.currentTarget.value)}
            placeholder="имя файла"
            size="xs"
            style={{ width: 160 }}
            styles={{
              input: {
                fontFamily: "var(--mantine-font-family-monospace)",
                fontSize: 12,
              },
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter") handleCreate();
              if (event.key === "Escape") cancelCreate();
            }}
            disabled={isCreatingFile}
          />
          <ActionIcon
            size="sm"
            variant="filled"
            loading={isCreatingFile}
            disabled={!newFileName.trim()}
            onClick={handleCreate}
            title="Создать"
          >
            <IconPlus size={13} />
          </ActionIcon>
          <ActionIcon
            size="sm"
            variant="subtle"
            onClick={cancelCreate}
            disabled={isCreatingFile}
            title="Отмена"
          >
            <IconX size={13} />
          </ActionIcon>
        </Group>
      ) : (
        <ActionIcon
          size="sm"
          variant="subtle"
          onClick={openCreateInput}
          title="Создать файл"
          style={{ flexShrink: 0 }}
        >
          <IconPlus size={14} />
        </ActionIcon>
      )}
    </div>
  );

  if (isLoadingFiles) {
    return (
      <Stack gap={0}>
        {fileBar}
        <Center py="xl">
          <Loader size="sm" />
        </Center>
      </Stack>
    );
  }

  if (leafFiles.length === 0) {
    return (
      <Stack gap={0}>
        {fileBar}
        <Center py="xl">
          <Stack align="center" gap="xs">
            <IconFile size={40} color="var(--mantine-color-dimmed)" />
            <Title order={5} c="dimmed">
              Нет файлов
            </Title>
          </Stack>
        </Center>
      </Stack>
    );
  }

  return (
    <Stack gap={0}>
      {fileBar}

      <Group
        px="md"
        py="xs"
        justify="space-between"
        style={{
          borderBottom: "1px solid var(--mantine-color-default-border)",
          flexShrink: 0,
        }}
      >
        <Group gap="xs">
          {selectedFile ? (
            <Code style={{ fontSize: 13 }}>{selectedFile}</Code>
          ) : (
            <Text size="sm" c="dimmed">
              Выберите файл
            </Text>
          )}
          {isLoadingFile && <Loader size="xs" />}
        </Group>
        <Group gap="xs">
          {selectedFile && setMain && (
            (() => {
              const selectedEntry = leafFiles.find(f => f.path === selectedFile);
              const isMain = selectedEntry?.is_main === true;
              return (
                <Button
                  size="xs"
                  variant={isMain ? "light" : "outline"}
                  color={isMain ? "yellow" : "gray"}
                  leftSection={<IconStar size={14} fill={isMain ? "currentColor" : "none"} />}
                  disabled={isMain || isSaving || isLoadingFile}
                  onClick={async () => {
                    const fileName = getFileName(selectedFile);
                    const [error] = await setMain(problemId, fileName);
                    if (error) {
                      notifications.show({
                        title: "Ошибка",
                        message: error.message ?? "Не удалось сделать файл основным",
                        color: "red",
                      });
                      return;
                    }
                    notifications.show({
                      title: "Успешно",
                      message: `${fileName} теперь используется как основной`,
                      color: "green",
                    });
                    mutateFiles();
                  }}
                >
                  {isMain ? "Основной" : "Сделать основным"}
                </Button>
              );
            })()
          )}
          {selectedFile && (
            <ActionIcon
              variant="subtle"
              title="Перезагрузить"
              disabled={isSaving || isLoadingFile}
              onClick={() => mutateContent()}
            >
              <IconRefresh size={16} />
            </ActionIcon>
          )}
          <Button
            size="xs"
            disabled={!selectedFile || !isDirty}
            loading={isSaving}
            onClick={handleSave}
          >
            Сохранить
          </Button>
        </Group>
      </Group>

      <Box
        style={{
          padding: "var(--mantine-spacing-xs)",
        }}
      >
        {selectedFile ? (
          <Textarea
            value={content}
            onChange={(event) => {
              setContent(event.currentTarget.value);
              setIsDirty(true);
            }}
            disabled={isLoadingFile || isSaving}
            autosize
            minRows={20}
            styles={{
              input: {
                fontFamily: "var(--mantine-font-family-monospace)",
                fontSize: 13,
                resize: "none",
                whiteSpace: "pre-wrap",
              },
            }}
          />
        ) : (
          <Center py="xl">
            <Stack align="center" gap="xs">
              <IconFile size={40} color="var(--mantine-color-dimmed)" />
              <Title order={5} c="dimmed">
                Выберите файл для редактирования
              </Title>
            </Stack>
          </Center>
        )}
      </Box>
    </Stack>
  );
}
