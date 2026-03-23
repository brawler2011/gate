"use client";

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
import { IconFile, IconPlus, IconRefresh, IconX } from "@tabler/icons-react";
import { useCallback, useEffect, useRef, useState, useTransition } from "react";
import type { FileEntry } from "@contracts/gateway/v1";
import { getWorkshopFile, listWorkshopFiles, saveWorkshopFile } from "@/lib/actions";
import classes from "./WorkshopFolderTab.module.css";

type Props = {
  problemId: string;
  folderName: string;
  selectedFile: string | null;
  onFileSelect: (filePath: string) => void;
  onFileCreated: (filePath: string) => void;
};

export function WorkshopFolderTab({
  problemId,
  folderName,
  selectedFile,
  onFileSelect,
  onFileCreated,
}: Props) {
  const [files, setFiles] = useState<FileEntry[]>([]);
  const [isLoadingFiles, setIsLoadingFiles] = useState(true);
  const [content, setContent] = useState<string>("");
  const [isDirty, setIsDirty] = useState(false);
  const [isLoadingFile, startLoadingFile] = useTransition();
  const [isSaving, startSaving] = useTransition();

  // New file creation state
  const [isCreating, setIsCreating] = useState(false);
  const [newFileName, setNewFileName] = useState("");
  const [isCreatingFile, startCreatingFile] = useTransition();
  const newFileInputRef = useRef<HTMLInputElement>(null);

  const fetchFiles = useCallback(async () => {
    setIsLoadingFiles(true);
    const [error, data] = await listWorkshopFiles(problemId, folderName);
    setIsLoadingFiles(false);
    if (error) {
      notifications.show({
        title: "Ошибка загрузки файлов",
        message: error.message ?? "Не удалось загрузить список файлов",
        color: "red",
      });
      return;
    }
    setFiles(data?.files ?? []);
  }, [problemId, folderName]);

  useEffect(() => {
    fetchFiles();
  }, [fetchFiles]);

  const leafFiles = files.filter((f) => !f.is_directory);

  const loadFile = useCallback(
    (path: string) => {
      startLoadingFile(async () => {
        const [error, data] = await getWorkshopFile(problemId, path);
        if (error) {
          notifications.show({
            title: "Ошибка загрузки файла",
            message: error.message ?? "Не удалось загрузить файл",
            color: "red",
          });
          return;
        }
        const text = typeof data === "string" ? data : "";
        setContent(text);
        setIsDirty(false);
      });
    },
    [problemId],
  );

  // Auto-open if single file or when selectedFile changes
  useEffect(() => {
    if (!selectedFile && leafFiles.length === 1) {
      onFileSelect(leafFiles[0].path!);
      return;
    }
    if (selectedFile) {
      loadFile(selectedFile);
    } else {
      setContent("");
      setIsDirty(false);
    }
  }, [selectedFile, folderName, leafFiles.length, loadFile, onFileSelect]);

  const handleSave = () => {
    if (!selectedFile) return;
    startSaving(async () => {
      const [error] = await saveWorkshopFile(problemId, selectedFile, content);
      if (error) {
        notifications.show({
          title: "Ошибка сохранения",
          message: error.message ?? "Не удалось сохранить файл",
          color: "red",
        });
        return;
      }
      setIsDirty(false);
      notifications.show({ title: "Сохранено", message: selectedFile, color: "green" });
    });
  };

  const getFileName = (path: string) => path.split("/").pop() ?? path;

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
      const [error] = await saveWorkshopFile(problemId, fullPath, "");
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
      await fetchFiles();
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
          {getFileName(file.path!)}
        </button>
      ))}

      {/* Inline new-file input */}
      {isCreating ? (
        <Group gap={4} style={{ flexShrink: 0 }}>
          <TextInput
            ref={newFileInputRef}
            value={newFileName}
            onChange={(e) => setNewFileName(e.currentTarget.value)}
            placeholder="имя файла"
            size="xs"
            style={{ width: 160 }}
            styles={{ input: { fontFamily: "var(--mantine-font-family-monospace)", fontSize: 12 } }}
            onKeyDown={(e) => {
              if (e.key === "Enter") handleCreate();
              if (e.key === "Escape") cancelCreate();
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

  if (leafFiles.length === 0) {
    return (
      <Stack gap={0} style={{ flex: 1, overflow: "hidden" }}>
        {fileBar}
        <Center style={{ flex: 1 }}>
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
    <Stack gap={0} style={{ flex: 1, height: "100%", overflow: "hidden" }}>
      {fileBar}

      {/* Toolbar */}
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
          {selectedFile && (
            <ActionIcon
              variant="subtle"
              title="Перезагрузить"
              disabled={isSaving || isLoadingFile}
              onClick={() => selectedFile && loadFile(selectedFile)}
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

      {/* Editor */}
      <Box style={{ flex: 1, overflow: "hidden", padding: "var(--mantine-spacing-xs)" }}>
        {selectedFile ? (
          <Textarea
            value={content}
            onChange={(e) => {
              setContent(e.currentTarget.value);
              setIsDirty(true);
            }}
            disabled={isLoadingFile || isSaving}
            autosize={false}
            styles={{
              wrapper: { height: "100%" },
              input: {
                height: "100%",
                fontFamily: "var(--mantine-font-family-monospace)",
                fontSize: 13,
                resize: "none",
                whiteSpace: "pre",
                overflowX: "auto",
              },
            }}
            style={{ height: "100%" }}
          />
        ) : (
          <Center h="100%">
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
