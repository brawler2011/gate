"use client";

import { highlightCode } from "@/lib/highlightCode";
import { APP_COLORS } from "@/lib/theme/colors";
import {
  ActionIcon,
  Button,
  Center,
  Group,
  Select,
  Stack,
  Text,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { IconPaperclip, IconTrash } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import dynamic from "next/dynamic";
import React, { useRef, useState } from "react";
import type CodeEditor from "react-simple-code-editor";
import classes from "./CreateSubmissionForm.module.css";
import "./vsc-dark-plus.css";

// Dynamic import to avoid hydration mismatch (autoCapitalize="off" vs "none")
type CodeEditorProps = React.ComponentProps<typeof CodeEditor>;

const Editor = dynamic(
  () => import("react-simple-code-editor").then((mod) => mod.default),
  { ssr: false },
);

const TypedEditor = Editor as React.ComponentType<CodeEditorProps>;

const languages = ["python", "cpp", "golang"];

const languageToExtension: Record<string, string> = {
  python: ".py",
  cpp: ".cpp",
  golang: ".go",
};

type Props = {
  onSubmit: (submission: FormData, language: string) => Promise<number | null>;
  problemSelect?: React.ReactNode;
  large?: boolean;
  disabled?: boolean;
};

const CreateSubmissionForm = ({
  onSubmit,
  problemSelect,
  large = false,
  disabled = false,
}: Props) => {
  const [file, setFile] = useState<File | null>(null);
  const [isDragging, setIsDragging] = useState(false);
  const [mounted, setMounted] = useState(false);
  const fileInputRef = useRef<HTMLInputElement>(null);

  React.useEffect(() => {
    setMounted(true);
  }, []);

  const form = useForm({
    initialValues: {
      code: "",
      language: languages[0],
    },
    validate: {
      code: (value) => (!value && !file ? "Code or file is required" : null),
      language: (value) =>
        languages.includes(value) ? null : "Invalid language",
    },
  });

  // React Query mutation for form submission
  const mutation = useMutation({
    mutationFn: async (values: typeof form.values) => {
      const formData = new FormData();
      if (file) {
        formData.append("submission", file);
      } else {
        formData.append("submission", values.code);
      }
      return await onSubmit(formData, values.language);
    },
    onSuccess: (data) => {
      if (data) {
        form.reset();
        setFile(null);
        if (fileInputRef.current) {
          fileInputRef.current.value = "";
        }
      }
    },
    onError: (error) => {
      console.error("Submission error:", error);
      // You can add notification here
    },
  });

  const handleFileSelect = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (disabled) return;
    const selectedFile = event.target.files?.[0];
    if (selectedFile) {
      setFile(selectedFile);
      form.setFieldValue("code", "");
    }
  };

  const handleDrop = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    if (disabled) {
      setIsDragging(false);
      return;
    }
    setIsDragging(false);

    const textData = event.dataTransfer.getData("text/plain");
    if (textData) {
      form.setFieldValue("code", textData);
      setFile(null);
      return;
    }

    const droppedFile = event.dataTransfer.files[0];
    if (droppedFile && /\.(py|cpp|go|txt)$/i.test(droppedFile.name)) {
      setFile(droppedFile);
      form.setFieldValue("code", "");
    }
  };

  const handleDragOver = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    if (!disabled) {
      setIsDragging(true);
    }
  };

  const handleDragLeave = () => {
    setIsDragging(false);
  };

  const removeFile = () => {
    if (disabled) return;
    setFile(null);
    if (fileInputRef.current) {
      fileInputRef.current.value = "";
    }
  };

  const handleKeyDown: React.KeyboardEventHandler<HTMLElement> = (event) => {
    if (event.key === "Tab") {
      if (!(event.currentTarget instanceof HTMLTextAreaElement)) {
        return;
      }

      event.preventDefault();
      const target = event.currentTarget;
      const start = target.selectionStart;
      const end = target.selectionEnd;
      const value = form.values.code;

      // Insert tab character at cursor position
      const newValue = value.substring(0, start) + "\t" + value.substring(end);
      form.setFieldValue("code", newValue);

      // Set cursor position after the inserted tab
      setTimeout(() => {
        target.selectionStart = target.selectionEnd = start + 1;
      }, 0);
    }
  };

  return (
    <form onSubmit={form.onSubmit((values) => mutation.mutate(values))}>
      <Stack
        className={`${classes.code} ${large ? classes.codeLarge : ""}`}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        data-dragging={isDragging}
      >
        <Group justify="space-between" gap="md" wrap="nowrap">
          <div style={{ flex: 1 }}>
            {problemSelect || (
              <Select
                data={languages}
                allowDeselect={false}
                variant="subtle"
                classNames={{ input: classes.languageSelectInput }}
                {...form.getInputProps("language")}
                style={{
                  width: `${(form.values.language?.length || 6) + 6}ch`,
                }}
                disabled={disabled}
              />
            )}
          </div>
          <div style={{ flex: 1, display: "flex", justifyContent: "center" }}>
            {problemSelect && (
              <Select
                data={languages}
                allowDeselect={false}
                variant="subtle"
                classNames={{ input: classes.languageSelectInput }}
                {...form.getInputProps("language")}
                style={{
                  width: `${(form.values.language?.length || 6) + 6}ch`,
                }}
                disabled={disabled}
              />
            )}
          </div>
          <div style={{ flex: 1, display: "flex", justifyContent: "flex-end" }}>
            <Button
              component="label"
              variant="subtle"
              leftSection={<IconPaperclip size={16} />}
              classNames={{
                label: classes.pinFileLabel,
                section: classes.pinFileSection,
                root: classes.pinFileRoot,
              }}
              disabled={disabled}
            >
              Файл
              <input
                type="file"
                hidden
                ref={fileInputRef}
                onChange={handleFileSelect}
                accept={languageToExtension[form.values.language]}
                disabled={disabled}
              />
            </Button>
          </div>
        </Group>

        <div className={classes.dropZone}>
          {isDragging ? (
            <Center className={classes.dragOverlay}>
              <Stack align="center" gap="xs">
                <Text c="blue" size="lg" ta="center">
                  Перетащите файл или текст сюда
                </Text>
                <Text c="dimmed" size="sm" ta="center">
                  Поддерживаются .py, .cpp, .go или обычный текст
                </Text>
              </Stack>
            </Center>
          ) : file ? (
            <div className={classes.fileAttached}>
              <Group className={classes.fileInfo}>
                <Text>Прикреплен файл: {file.name}</Text>
              </Group>
              <ActionIcon
                onClick={removeFile}
                color="red"
                className={classes.deleteButton}
                variant="subtle"
                disabled={disabled}
              >
                <IconTrash size={20} />
              </ActionIcon>
            </div>
          ) : (
            <div className={classes.editorContainer}>
              {mounted && (
                <TypedEditor
                  value={form.values.code}
                  onValueChange={(code: string) =>
                    form.setFieldValue("code", code)
                  }
                  highlight={(code: string) =>
                    highlightCode(code, form.values.language)
                  }
                  padding={10}
                  placeholder="Введите ваше решение здесь, перетащите файл или текст..."
                  className={classes.codeEditor}
                  disabled={disabled}
                  textareaId="code-editor-textarea"
                  onKeyDown={handleKeyDown}
                />
              )}
            </div>
          )}
        </div>

        <Button
          type="submit"
          loading={mutation.isPending}
          disabled={disabled}
          color={APP_COLORS.submissions}
        >
          Отправить решение
        </Button>
      </Stack>
    </form>
  );
};

export { CreateSubmissionForm };
