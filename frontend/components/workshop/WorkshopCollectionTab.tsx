"use client";

import {
  ActionIcon,
  Button,
  Center,
  Code,
  Group,
  Loader,
  Modal,
  Stack,
  Text,
  TextInput,
  Textarea,
  Title,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconFile, IconPlus, IconRefresh} from "@tabler/icons-react";
import {useEffect, useRef, useState, useTransition} from "react";
import useSWR from "swr";

import classes from "./WorkshopFolderTab.module.css";

import type {FileEntry} from "@/contracts/core/v1";
import type {ApiError} from "@/lib/api";

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

export const WorkshopCollectionTab = ({
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
}: Props) => {
  const [content, setContent] = useState<string>("");
  const [isDirty, setIsDirty] = useState(false);
  const [isSaving, startSaving] = useTransition();

  const [isCreating, setIsCreating] = useState(false);
  const [newFileName, setNewFileName] = useState("");
  const [isCreatingFile, startCreatingFile] = useTransition();
  const newFileInputRef = useRef<HTMLInputElement>(null);

  const getFileName = (path: string) => path.split("/").pop() ?? path;

  const {data: filesData, isLoading: isLoadingFiles, mutate: mutateFiles} = useSWR(
    ["workshop-files", problemId, folderName],
    async () => {
      const [err, res] = await listFiles(problemId);
      if (err) {
        throw new Error(err.message || "Не удалось загрузить список файлов");
      }
      return res;
    }
  );

  const files = filesData?.files || [];

  const fileName = selectedFile ? getFileName(selectedFile) : null;
  const {data: fileContent, isLoading: isLoadingFile, mutate: mutateContent} = useSWR(
    fileName ? ["workshop-file-content", problemId, folderName, fileName] : null,
    async () => {
      const [err, res] = await getFile(problemId, fileName!);
      if (err) {
        throw new Error(err.message || "Не удалось загрузить файл");
      }
      return res;
    }
  );

  const leafFiles = files.filter((file) => !file.is_directory && file.path !== "tests.json");

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
    if (!selectedFile) {
      return;
    }

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
    if (!trimmed) {
      return;
    }

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

  return (
    <div className={classes.container}>
      {/* Modal for creating files */}
      <Modal
        opened={isCreating}
        onClose={cancelCreate}
        title="Добавить файл"
        centered
        size="sm"
      >
        <Stack gap="md">
          <TextInput
            ref={newFileInputRef}
            label="Имя файла"
            value={newFileName}
            onChange={(event) => setNewFileName(event.currentTarget.value)}
            placeholder="например: checker.cpp"
            data-autofocus
            styles={{
              input: {
                fontFamily: "var(--mantine-font-family-monospace)",
              },
            }}
            onKeyDown={(event) => {
              if (event.key === "Enter") {
                handleCreate();
              }
              if (event.key === "Escape") {
                cancelCreate();
              }
            }}
            disabled={isCreatingFile}
          />
          <Group gap="xs" justify="flex-end">
            <Button
              variant="subtle"
              color="gray"
              onClick={cancelCreate}
              disabled={isCreatingFile}
            >
              Отмена
            </Button>
            <Button
              loading={isCreatingFile}
              disabled={!newFileName.trim()}
              onClick={handleCreate}
              leftSection={<IconPlus size={16} />}
            >
              Создать
            </Button>
          </Group>
        </Stack>
      </Modal>

      {/* Left Sidebar - Files list & Add button */}
      <div className={classes.sidebar}>
        <Button
          size="sm"
          variant="light"
          leftSection={<IconPlus size={16} />}
          onClick={openCreateInput}
          fullWidth
        >
          Добавить
        </Button>

        <div className={classes.fileList}>
          {isLoadingFiles && (
            <Center py="xl">
              <Loader size="sm" />
            </Center>
          )}
          {!isLoadingFiles && leafFiles.length === 0 && (
            <div className={classes.sidebarEmptyText}>нет файлов</div>
          )}
          {!isLoadingFiles && leafFiles.length > 0 && (
            leafFiles.map((file) => {
              const isMain = file.is_main;
              const isActive = selectedFile === file.path;

              let itemClassName = classes.fileItem;
              if (isMain && isActive) {
                itemClassName = `${classes.fileItem} ${classes.fileItemMainActive}`;
              } else if (isMain) {
                itemClassName = `${classes.fileItem} ${classes.fileItemMain}`;
              } else if (isActive) {
                itemClassName = `${classes.fileItem} ${classes.fileItemActive}`;
              }

              return (
                <button
                  key={file.path}
                  type="button"
                  className={itemClassName}
                  onClick={() => {
                    if (selectedFile !== file.path) {
                      onFileSelect(file.path!);
                    }
                  }}
                >
                  <span>{getFileName(file.path!)}</span>
                </button>
              );
            })
          )}
        </div>
      </div>

      {/* Right Column - Editor Area */}
      <div className={classes.editorArea}>
        {/* Editor Toolbar Header */}
        <div className={classes.editorHeader}>
          <Group gap="xs">
            {selectedFile ? (
              <Code style={{fontSize: 13}}>{selectedFile}</Code>
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
        </div>

        {/* Editor Content Box */}
        <div className={classes.editorWrapper}>
          {selectedFile ? (
            <Textarea
              value={content}
              onChange={(event) => {
                setContent(event.currentTarget.value);
                setIsDirty(true);
              }}
              disabled={isLoadingFile || isSaving}
              autosize
              minRows={25}
              styles={{
                input: {
                  fontFamily: "var(--mantine-font-family-monospace)",
                  fontSize: 13,
                  resize: "none",
                  whiteSpace: "pre-wrap",
                  flexGrow: 1,
                },
              }}
            />
          ) : (
            <div className={classes.emptyState}>
              <Stack align="center" gap="xs">
                <IconFile size={40} color="var(--mantine-color-dimmed)" />
                <Title order={5} c="dimmed">
                  Выберите файл для редактирования
                </Title>
              </Stack>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
