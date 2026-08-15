"use client";

import {
  ActionIcon,
  Button,
  Center,
  Code,
  Group,
  Loader,
  Modal,
  Select,
  Stack,
  Text,
  TextInput,
  Textarea,
  Title,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconFile, IconPlus, IconRefresh, IconTrash} from "@tabler/icons-react";
import {useEffect, useRef, useState, useTransition} from "react";
import useSWR from "swr";

import {api} from "@/lib/api";
import classes from "./WorkshopFolderTab.module.css";

import type {FileEntry} from "@/contracts/core/v1";
import type {ApiError} from "@/lib/api";
import type {ReactNode} from "react";

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
  deleteFile?: (
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
  deleteFile,
}: Props): ReactNode => {
  const [content, setContent] = useState<string>("");
  const [isDirty, setIsDirty] = useState(false);
  const [isSaving, startSaving] = useTransition();

  const [isCreating, setIsCreating] = useState(false);
  const [newFileName, setNewFileName] = useState("");
  const [selectedExt, setSelectedExt] = useState<string>("cpp");
  const [isCreatingFile, startCreatingFile] = useTransition();
  const newFileInputRef = useRef<HTMLInputElement>(null);

  const [isDeleteModalOpen, setIsDeleteModalOpen] = useState(false);
  const [isDeleting, startDeleting] = useTransition();

  const getFileName = (path: string) => path.split("/").pop() ?? path;

  // Fetch languages dynamically from backend API
  const {data: languagesData} = useSWR("supported-languages", async () => {
    const [err, res] = await api.getLanguages();
    if (err || !res?.languages) {
      return null;
    }
    return res.languages;
  });

  const baseLangOptions = languagesData?.length
    ? languagesData.map((lang) => {
        const isCpp = lang.name.toLowerCase() === "cpp" || lang.extension === "cc" || lang.extension === "cpp";
        const ext = isCpp ? "cpp" : lang.extension;
        const labelName = isCpp ? "C++" : lang.name.toUpperCase();
        return {
          value: ext,
          label: `${labelName} (.${ext})`,
        };
      })
    : [
        {value: "cpp", label: "C++ (.cpp)"},
        {value: "py", label: "Python (.py)"},
        {value: "go", label: "Go (.go)"},
        {value: "java", label: "Java (.java)"},
      ];

  const extensionOptions =
    folderName === "lib"
      ? [
          ...baseLangOptions,
          {value: "h", label: "Header (.h)"},
          {value: "hpp", label: "Header (.hpp)"},
          {value: "inc", label: "Include (.inc)"},
        ]
      : baseLangOptions;

  const uniqueExtensionOptions = extensionOptions.filter(
    (opt, index, self) => self.findIndex((o) => o.value === opt.value) === index
  );

  const allowedExtensions = uniqueExtensionOptions.map((o) => o.value);

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

  const leafFiles = files.filter(
    (file) => !file.is_directory && file.path && file.path !== "tests.json" && !getFileName(file.path).startsWith(".")
  );

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
    setSelectedExt("cpp");
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

    let base = trimmed;
    let ext = selectedExt;

    if (trimmed.includes(".")) {
      const parts = trimmed.split(".");
      const inputExt = parts.pop()?.toLowerCase() || "";
      const inputBase = parts.join(".");

      if (allowedExtensions.includes(inputExt)) {
        base = inputBase;
        ext = inputExt;
      } else {
        notifications.show({
          title: "Недопустимое расширение",
          message: `Расширение .${inputExt} недопустимо. Допустимые расширения: ${allowedExtensions.map((e) => `.${e}`).join(", ")}`,
          color: "red",
        });
        return;
      }
    }

    const finalFileName = `${base}.${ext}`;
    const fullPath = `${folderName}/${finalFileName}`;

    startCreatingFile(async () => {
      const [error] = await createFile(problemId, finalFileName, "");
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

  const handleDelete = () => {
    if (!selectedFile || !deleteFile) {
      return;
    }

    startDeleting(async () => {
      const targetFileName = getFileName(selectedFile);
      const [error] = await deleteFile(problemId, targetFileName);

      if (error) {
        notifications.show({
          title: "Ошибка удаления",
          message: error.message ?? "Не удалось удалить файл",
          color: "red",
        });
        return;
      }

      setIsDeleteModalOpen(false);
      setContent("");
      setIsDirty(false);
      notifications.show({
        title: "Удалено",
        message: `Файл ${targetFileName} был удален`,
        color: "blue",
      });
      onFileSelect("");
      mutateFiles();
    });
  };

  return (
    <div className={classes.container}>
      {/* Modal for creating files */}
      <Modal
        opened={isCreating}
        onClose={cancelCreate}
        title={folderName === "lib" ? "Добавить файл библиотеки" : "Добавить решение"}
        centered
        size="sm"
      >
        <Stack gap="md">
          <TextInput
            ref={newFileInputRef}
            label="Имя файла"
            value={newFileName}
            onChange={(event) => setNewFileName(event.currentTarget.value)}
            placeholder={folderName === "lib" ? "например: testlib" : "например: solution"}
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
          <Select
            label="Расширение файла"
            value={selectedExt}
            onChange={(val) => setSelectedExt(val || "cpp")}
            data={uniqueExtensionOptions}
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

      {/* Modal for deleting files */}
      {deleteFile && selectedFile && (
        <Modal
          opened={isDeleteModalOpen}
          onClose={() => setIsDeleteModalOpen(false)}
          title="Подтверждение удаления"
          centered
          size="sm"
        >
          <Stack gap="md">
            <Text size="sm">
              Вы действительно хотите удалить файл <Code>{getFileName(selectedFile)}</Code>?
            </Text>
            <Group gap="xs" justify="flex-end">
              <Button
                variant="subtle"
                color="gray"
                onClick={() => setIsDeleteModalOpen(false)}
                disabled={isDeleting}
              >
                Отмена
              </Button>
              <Button
                color="red"
                loading={isDeleting}
                onClick={handleDelete}
              >
                Удалить
              </Button>
            </Group>
          </Stack>
        </Modal>
      )}

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
            {selectedFile && deleteFile && (
              <ActionIcon
                variant="subtle"
                color="red"
                title="Удалить файл"
                disabled={isSaving || isLoadingFile || isDeleting}
                onClick={() => setIsDeleteModalOpen(true)}
              >
                <IconTrash size={16} />
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
