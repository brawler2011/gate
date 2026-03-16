"use client";

import {
  ActionIcon,
  Box,
  Button,
  Center,
  Code,
  Group,
  Loader,
  NavLink,
  ScrollArea,
  Stack,
  Text,
  Textarea,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconFile, IconFolder, IconRefresh } from "@tabler/icons-react";
import { useState, useTransition } from "react";
import type { FileEntry } from "@contracts/gateway/v1";
import { getWorkshopFile, saveWorkshopFile } from "@/lib/actions";

type Props = {
  problemId: string;
  initialFiles: FileEntry[];
};

export function WorkshopEditor({ problemId, initialFiles }: Props) {
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [content, setContent] = useState<string>("");
  const [isDirty, setIsDirty] = useState(false);
  const [isLoadingFile, startLoadingFile] = useTransition();
  const [isSaving, startSaving] = useTransition();

  const handleSelectFile = (file: FileEntry) => {
    if (!file.path || file.is_directory) return;

    startLoadingFile(async () => {
      const [error, data] = await getWorkshopFile(problemId, file.path!);
      if (error) {
        notifications.show({
          title: "Ошибка загрузки файла",
          message: error.message ?? "Не удалось загрузить файл",
          color: "red",
        });
        return;
      }
      // data arrives as string at runtime (response.text() for non-JSON content)
      const text = typeof data === "string" ? data : "";
      setSelectedPath(file.path!);
      setContent(text);
      setIsDirty(false);
    });
  };

  const handleSave = () => {
    if (!selectedPath) return;

    startSaving(async () => {
      const [error] = await saveWorkshopFile(problemId, selectedPath, content);
      if (error) {
        notifications.show({
          title: "Ошибка сохранения",
          message: error.message ?? "Не удалось сохранить файл",
          color: "red",
        });
        return;
      }
      setIsDirty(false);
      notifications.show({
        title: "Сохранено",
        message: selectedPath,
        color: "green",
      });
    });
  };

  const sortedFiles = [...initialFiles].sort((a, b) => {
    if (a.is_directory === b.is_directory) {
      return (a.path ?? "").localeCompare(b.path ?? "");
    }
    return a.is_directory ? -1 : 1;
  });

  return (
    <Group align="flex-start" gap={0} style={{ height: "calc(100vh - 120px)", minHeight: 400 }}>
      {/* File tree */}
      <Box
        style={{
          width: 240,
          flexShrink: 0,
          borderRight: "1px solid var(--mantine-color-default-border)",
          height: "100%",
        }}
      >
        <Box
          px="sm"
          py="xs"
          style={{ borderBottom: "1px solid var(--mantine-color-default-border)" }}
        >
          <Text size="xs" fw={600} c="dimmed" tt="uppercase">
            Файлы
          </Text>
        </Box>
        <ScrollArea h="calc(100% - 36px)">
          {sortedFiles.length === 0 ? (
            <Center py="lg">
              <Text size="sm" c="dimmed">
                Нет файлов
              </Text>
            </Center>
          ) : (
            sortedFiles.map((file) => (
              <NavLink
                key={file.path}
                label={file.path}
                leftSection={
                  file.is_directory ? (
                    <IconFolder size={14} />
                  ) : (
                    <IconFile size={14} />
                  )
                }
                active={selectedPath === file.path}
                disabled={!!file.is_directory}
                onClick={() => handleSelectFile(file)}
                styles={{ label: { fontFamily: "var(--mantine-font-family-monospace)", fontSize: 13 } }}
              />
            ))
          )}
        </ScrollArea>
      </Box>

      {/* Editor panel */}
      <Stack gap={0} style={{ flex: 1, height: "100%", overflow: "hidden" }}>
        {/* Toolbar */}
        <Group
          px="md"
          py="xs"
          justify="space-between"
          style={{ borderBottom: "1px solid var(--mantine-color-default-border)", flexShrink: 0 }}
        >
          <Group gap="xs">
            {selectedPath ? (
              <Code style={{ fontSize: 13 }}>{selectedPath}</Code>
            ) : (
              <Text size="sm" c="dimmed">
                Выберите файл
              </Text>
            )}
            {isLoadingFile && <Loader size="xs" />}
          </Group>
          <Group gap="xs">
            {selectedPath && (
              <ActionIcon
                variant="subtle"
                title="Перезагрузить"
                disabled={isSaving || isLoadingFile}
                onClick={() => {
                  if (!selectedPath) return;
                  handleSelectFile({ path: selectedPath, is_directory: false });
                }}
              >
                <IconRefresh size={16} />
              </ActionIcon>
            )}
            <Button
              size="xs"
              disabled={!selectedPath || !isDirty}
              loading={isSaving}
              onClick={handleSave}
            >
              Сохранить
            </Button>
          </Group>
        </Group>

        {/* Text area */}
        <Box style={{ flex: 1, overflow: "hidden", padding: "var(--mantine-spacing-xs)" }}>
          {selectedPath ? (
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
    </Group>
  );
}
