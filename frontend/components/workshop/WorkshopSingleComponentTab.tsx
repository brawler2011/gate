"use client";

import {
  ActionIcon,
  Button,
  Center,
  Code,
  Group,
  Loader,
  Modal,
  Paper,
  Select,
  Stack,
  Text,
  Textarea,
  Title,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconCheck, IconCode, IconPlayerPlay, IconTrash} from "@tabler/icons-react";
import {useEffect, useState, useTransition} from "react";
import useSWR from "swr";

import {api, type ApiError} from "@/lib/api";

import classes from "./WorkshopFolderTab.module.css";

import type {FileEntry} from "@/contracts/core/v1";
import type {ReactNode} from "react";

type ListFilesResult = Promise<[ApiError | null, { files?: FileEntry[] } | null]>;
type GetFileResult = Promise<[ApiError | null, string | null]>;
type SaveFileResult = Promise<[ApiError | null, unknown | null]>;

type Props = {
  problemId: string;
  componentType: "checker" | "generator" | "interactor" | "validator";
  componentTitle: string;
  defaultFileName: string;
  listFiles: (problemId: string) => ListFilesResult;
  getFile: (problemId: string, name: string) => GetFileResult;
  createFile: (problemId: string, name: string, content: string) => SaveFileResult;
  updateFile: (problemId: string, name: string, content: string) => SaveFileResult;
  deleteFile: (problemId: string, name: string) => SaveFileResult;
};

const DEFAULT_LANGUAGE_OPTIONS = [
  {value: "cpp", label: "C++ (.cpp)"},
  {value: "py", label: "Python (.py)"},
  {value: "go", label: "Go (.go)"},
  {value: "java", label: "Java (.java)"},
];

const DEFAULT_TEMPLATES: Record<string, Record<string, string>> = {
  checker: {
    cpp: `#include "testlib.h"\n\nint main(int argc, char* argv[]) {\n    setName("checker");\n    registerTestlibCmd(argc, argv);\n\n    // TODO: implement checker logic\n    quitf(_ok, "correct");\n}\n`,
    cc: `#include "testlib.h"\n\nint main(int argc, char* argv[]) {\n    setName("checker");\n    registerTestlibCmd(argc, argv);\n\n    // TODO: implement checker logic\n    quitf(_ok, "correct");\n}\n`,
    py: `import sys\n\ndef main():\n    # TODO: implement checker logic\n    sys.exit(0)\n\nif __name__ == "__main__":\n    main()\n`,
    go: `package main\n\nimport "os"\n\nfunc main() {\n    // TODO: implement checker logic\n    os.Exit(0)\n}\n`,
    java: `public class Main {\n    public static void main(String[] args) {\n        // TODO: implement checker logic\n    }\n}\n`,
  },
  generator: {
    cpp: `#include "testlib.h"\n\nint main(int argc, char* argv[]) {\n    registerGen(argc, argv, 1);\n\n    // TODO: implement generator logic\n    return 0;\n}\n`,
    cc: `#include "testlib.h"\n\nint main(int argc, char* argv[]) {\n    registerGen(argc, argv, 1);\n\n    // TODO: implement generator logic\n    return 0;\n}\n`,
    py: `import sys\n\ndef main():\n    # TODO: implement generator logic\n    pass\n\nif __name__ == "__main__":\n    main()\n`,
    go: `package main\n\nfunc main() {\n    // TODO: implement generator logic\n}\n`,
    java: `public class Main {\n    public static void main(String[] args) {\n        // TODO: implement generator logic\n    }\n}\n`,
  },
  interactor: {
    cpp: `#include "testlib.h"\n\nint main(int argc, char* argv[]) {\n    setName("interactor");\n    registerInteraction(argc, argv);\n\n    // TODO: implement interactor logic\n    quitf(_ok, "solved");\n}\n`,
    cc: `#include "testlib.h"\n\nint main(int argc, char* argv[]) {\n    setName("interactor");\n    registerInteraction(argc, argv);\n\n    // TODO: implement interactor logic\n    quitf(_ok, "solved");\n}\n`,
    py: `import sys\n\ndef main():\n    # TODO: implement interactor logic\n    pass\n\nif __name__ == "__main__":\n    main()\n`,
    go: `package main\n\nfunc main() {\n    // TODO: implement interactor logic\n}\n`,
    java: `public class Main {\n    public static void main(String[] args) {\n        // TODO: implement interactor logic\n    }\n}\n`,
  },
  validator: {
    cpp: `#include "testlib.h"\n\nint main(int argc, char* argv[]) {\n    registerValidation(argc, argv);\n\n    // TODO: implement validator logic\n    inf.readEof();\n    return 0;\n}\n`,
    cc: `#include "testlib.h"\n\nint main(int argc, char* argv[]) {\n    registerValidation(argc, argv);\n\n    // TODO: implement validator logic\n    inf.readEof();\n    return 0;\n}\n`,
    py: `import sys\n\ndef main():\n    # TODO: implement validator logic\n    pass\n\nif __name__ == "__main__":\n    main()\n`,
    go: `package main\n\nfunc main() {\n    // TODO: implement validator logic\n}\n`,
    java: `public class Main {\n    public static void main(String[] args) {\n        // TODO: implement validator logic\n    }\n}\n`,
  },
};

export const WorkshopSingleComponentTab = ({
  problemId,
  componentType,
  componentTitle,
  defaultFileName,
  listFiles,
  getFile,
  createFile,
  updateFile,
  deleteFile,
}: Props): ReactNode => {
  const [content, setContent] = useState<string>("");
  const [isDirty, setIsDirty] = useState(false);
  const [isSaving, startSaving] = useTransition();
  const [isCompiling, startCompiling] = useTransition();
  const [selectedExt, setSelectedExt] = useState<string>("cpp");
  const [isCreating, startCreating] = useTransition();
  const [isDeletingModalOpen, setIsDeletingModalOpen] = useState(false);
  const [isDeleting, startDeleting] = useTransition();

  const getFileName = (path: string) => path.split("/").pop() ?? path;

  // Fetch languages dynamically from backend API (which reads languages.yaml)
  const {data: languagesData} = useSWR("supported-languages", async () => {
    const [err, res] = await api.getLanguages();
    if (err || !res?.languages) {
      return null;
    }
    return res.languages;
  });

  const languageOptions = languagesData?.length
    ? languagesData.map((lang) => {
        const isCpp = lang.name.toLowerCase() === "cpp" || lang.extension === "cc" || lang.extension === "cpp";
        const ext = isCpp ? "cpp" : lang.extension;
        const labelName = isCpp ? "C++" : lang.name.toUpperCase();
        return {
          value: ext,
          label: `${labelName} (.${ext})`,
        };
      })
    : DEFAULT_LANGUAGE_OPTIONS;

  useEffect(() => {
    if (languageOptions.length > 0 && !languageOptions.some((opt) => opt.value === selectedExt)) {
      const cppOpt = languageOptions.find((opt) => opt.value === "cpp" || opt.value === "cc");
      setSelectedExt(cppOpt ? cppOpt.value : languageOptions[0].value);
    }
  }, [languageOptions, selectedExt]);

  const {data: filesData, isLoading: isLoadingFiles, mutate: mutateFiles} = useSWR(
    ["workshop-single-component-files", problemId, componentType],
    async () => {
      const [err, res] = await listFiles(problemId);
      if (err) {
        throw new Error(err.message || "Не удалось загрузить компонент");
      }
      return res;
    }
  );

  const leafFiles = (filesData?.files || []).filter(
    (f) => !f.is_directory && f.path && !getFileName(f.path).startsWith(".")
  );
  const activeFile = leafFiles.length > 0 ? leafFiles[0] : null;
  const activeFileName = activeFile?.path ? getFileName(activeFile.path) : null;

  const {data: fileContent, isLoading: isLoadingContent, mutate: mutateContent} = useSWR(
    activeFileName ? ["workshop-single-component-content", problemId, componentType, activeFileName] : null,
    async () => {
      const [err, res] = await getFile(problemId, activeFileName!);
      if (err) {
        throw new Error(err.message || "Не удалось загрузить содержимое компонента");
      }
      return res;
    }
  );

  useEffect(() => {
    if (fileContent !== undefined && activeFileName) {
      setContent(fileContent || "");
      setIsDirty(false);
    }
  }, [fileContent, activeFileName]);

  const handleSave = () => {
    if (!activeFileName) return;
    startSaving(async () => {
      const [error] = await updateFile(problemId, activeFileName, content);
      if (error) {
        notifications.show({
          title: "Ошибка сохранения",
          message: error.message ?? "Не удалось сохранить компонент",
          color: "red",
        });
        return;
      }
      setIsDirty(false);
      await mutateContent();
      notifications.show({
        title: "Сохранено",
        message: `${componentTitle} успешно сохранен`,
        color: "green",
      });
    });
  };

  const handleCreate = () => {
    const ext = selectedExt || "cpp";
    const name = `${defaultFileName}.${ext}`;
    const initialCode = DEFAULT_TEMPLATES[componentType]?.[ext] || "// TODO: write code\n";

    startCreating(async () => {
      const [error] = await createFile(problemId, name, initialCode);
      if (error) {
        notifications.show({
          title: "Ошибка создания",
          message: error.message ?? "Не удалось создать компонент",
          color: "red",
        });
        return;
      }
      await mutateFiles();
      notifications.show({
        title: "Создано",
        message: `${componentTitle} создан (${name})`,
        color: "green",
      });
    });
  };

  const handleDelete = () => {
    if (!activeFileName) return;
    startDeleting(async () => {
      const [error] = await deleteFile(problemId, activeFileName);
      if (error) {
        notifications.show({
          title: "Ошибка удаления",
          message: error.message ?? "Не удалось удалить компонент",
          color: "red",
        });
        return;
      }
      setIsDeletingModalOpen(false);
      setContent("");
      setIsDirty(false);
      await mutateFiles();
      notifications.show({
        title: "Удалено",
        message: `${componentTitle} удален`,
        color: "blue",
      });
    });
  };

  const handleCompile = () => {
    startCompiling(async () => {
      const [error, res] = await api.compileProblemComponent({
        problemId,
        componentType,
      });

      if (error) {
        notifications.show({
          title: "Ошибка компиляции",
          message: error.message ?? "Не удалось отправить на компиляцию",
          color: "red",
        });
        return;
      }

      if (res?.success) {
        notifications.show({
          title: "Успешная компиляция",
          message: `${componentTitle} успешно скомпилирован`,
          color: "green",
        });
      } else {
        notifications.show({
          title: "Ошибка компиляции",
          message: res?.compile_error || "Компиляция завершилась с ошибкой",
          color: "red",
        });
      }
    });
  };

  if (isLoadingFiles) {
    return (
      <Center h={300}>
        <Loader size="md" />
      </Center>
    );
  }

  // Empty state: component does not exist yet
  if (!activeFile) {
    return (
      <Center h={400}>
        <Paper p="xl" withBorder radius="md" style={{maxWidth: 400, width: "100%"}}>
          <Stack align="center" gap="md">
            <IconCode size={40} stroke={1.5} color="var(--mantine-color-dimmed)" />
            <Title order={4}>{componentTitle}</Title>
            <Select
              label="Язык программирования"
              value={selectedExt}
              onChange={(val) => setSelectedExt(val || "cpp")}
              data={languageOptions}
              w="100%"
            />
            <Button
              fullWidth
              loading={isCreating}
              onClick={handleCreate}
              leftSection={<IconCheck size={16} />}
            >
              Создать {componentTitle.toLowerCase()}
            </Button>
          </Stack>
        </Paper>
      </Center>
    );
  }

  return (
    <Stack gap={0} className={classes.root}>
      {/* Action Header */}
      <Paper p="md" withBorder radius={0} className={classes.header}>
        <Group justify="space-between">
          <Group gap="sm">
            <IconCode size={20} />
            <Text fw={600} size="md">
              {activeFile.path}
            </Text>
            {isDirty && (
              <Code color="yellow" dir="ltr">
                Не сохранено
              </Code>
            )}
          </Group>

          <Group gap="xs">
            <Button
              size="xs"
              variant="default"
              loading={isCompiling}
              onClick={handleCompile}
              leftSection={<IconPlayerPlay size={14} />}
            >
              Скомпилировать
            </Button>

            <Button
              size="xs"
              color="blue"
              loading={isSaving}
              disabled={!isDirty}
              onClick={handleSave}
              leftSection={<IconCheck size={14} />}
            >
              Сохранить
            </Button>

            <ActionIcon
              size="input-xs"
              color="red"
              variant="light"
              onClick={() => setIsDeletingModalOpen(true)}
              title="Удалить компонент"
            >
              <IconTrash size={16} />
            </ActionIcon>
          </Group>
        </Group>
      </Paper>

      {/* Editor area */}
      <Stack gap={0} p="md" style={{flex: 1}}>
        {isLoadingContent ? (
          <Center h={300}>
            <Loader size="md" />
          </Center>
        ) : (
          <Textarea
            value={content}
            onChange={(e) => {
              setContent(e.currentTarget.value);
              setIsDirty(true);
            }}
            styles={{
              input: {
                fontFamily: "monospace",
                fontSize: 14,
                minHeight: "calc(100vh - 220px)",
                lineHeight: 1.5,
              },
            }}
            autosize={false}
          />
        )}
      </Stack>

      {/* Delete Confirmation Modal */}
      <Modal
        opened={isDeletingModalOpen}
        onClose={() => setIsDeletingModalOpen(false)}
        title={`Удалить ${componentTitle.toLowerCase()}?`}
        centered
      >
        <Stack gap="md">
          <Text size="sm">
            Вы уверены, что хотите удалить {componentTitle.toLowerCase()} (
            <Code>{activeFile.path}</Code>)? Данное действие нельзя отменить.
          </Text>
          <Group justify="flex-end" gap="xs">
            <Button variant="default" onClick={() => setIsDeletingModalOpen(false)}>
              Отмена
            </Button>
            <Button color="red" loading={isDeleting} onClick={handleDelete}>
              Удалить
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Stack>
  );
};
