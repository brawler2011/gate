"use client";

import {ActionIcon, Button, Center, Group, Select, Stack, Text} from "@mantine/core";
import {useForm} from "@mantine/form";
import {IconPaperclip, IconTrash} from "@tabler/icons-react";
import React, {useRef, useState, useTransition, type ReactNode} from "react";
import Editor from "react-simple-code-editor";

import {detectSubmissionLanguage} from "@/lib/detectSubmissionLanguage";
import {highlightCode} from "@/lib/highlightCode";
import {APP_COLORS} from "@/lib/theme/colors";

import classes from "./CreateSubmissionForm.module.css";

import type CodeEditor from "react-simple-code-editor";

import "./vsc-dark-plus.css";

// Dynamic import to avoid hydration mismatch (autoCapitalize="off" vs "none")
type CodeEditorProps = React.ComponentProps<typeof CodeEditor>;

const TypedEditor = Editor as React.ComponentType<CodeEditorProps>;

const languages = ["auto", "python", "cpp", "golang"];

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
}: Props): ReactNode => {
  const [automaticLanguage, setAutomaticLanguage] = useState(true);
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
        languages.includes(value) && value !== "auto"
          ? null : "Не удалось определить язык. Выберите его вручную.",
    },
  });

  const {setFieldValue} = form;
  React.useEffect(() => {
    if (!automaticLanguage) {
      return;
    }
    let cancelled = false;
    const timer = setTimeout(async () => {
      try {
        const source = file ? await file.slice(0, 32768).text() : form.values.code;
        if (!cancelled) {
          setFieldValue("language", detectSubmissionLanguage(source) ?? "auto");
        }
      } catch {
        if (!cancelled) {
          setFieldValue("language", "auto");
        }
      }
    }, 200);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [automaticLanguage, file, form.values.code, form.values.language, setFieldValue]);

  const setCode = (code: string) => {
    form.setFieldValue("code", code);
    if (automaticLanguage) {
      form.setFieldValue("language", "auto");
    }
  };

  const selectLanguage = (language: string | null) => {
    if (language) {
      setAutomaticLanguage(language === "auto");
      form.setFieldValue("language", language);
    }
  };

  const [isPending, startTransition] = useTransition();

  const onSubmitHandler = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    form.onSubmit((values) => {
      startTransition(async () => {
        try {
          const formData = new FormData();
          if (file) {
            formData.append("submission", file);
          } else {
            formData.append("submission", values.code);
          }
          const data = await onSubmit(formData, values.language);
          if (data) {
            form.reset();
            setAutomaticLanguage(true);
            setFile(null);
            if (fileInputRef.current) {
              fileInputRef.current.value = "";
            }
          }
        } catch (error) {
          console.error("Submission error:", error);
        }
      });
    })(event);
  };

  const handleFileSelect = (event: React.ChangeEvent<HTMLInputElement>) => {
    if (disabled) {
      return;
    }
    const selectedFile = event.target.files?.[0];
    if (selectedFile) {
      setFile(selectedFile);
      setCode("");
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
      setCode(textData);
      setFile(null);
      return;
    }

    const droppedFile = event.dataTransfer.files[0];
    if (droppedFile && /\.(py|cpp|go|txt)$/i.test(droppedFile.name)) {
      setFile(droppedFile);
      setCode("");
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
    if (disabled) {
      return;
    }
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
      setCode(newValue);

      // Set cursor position after the inserted tab
      setTimeout(() => {
        target.selectionStart = target.selectionEnd = start + 1;
      }, 0);
    }
  };

  return (
    <form onSubmit={onSubmitHandler}>
      <Stack
        className={`${classes.code} ${large ? classes.codeLarge : ""}`}
        onDrop={handleDrop}
        onDragOver={handleDragOver}
        onDragLeave={handleDragLeave}
        data-dragging={isDragging}
      >
        <Group justify="space-between" gap="md" wrap="nowrap">
          <div style={{flex: 1}}>
            {problemSelect || (
              <Select
                data={languages}
                aria-label="Язык программирования"
                allowDeselect={false}
                variant="subtle"
                classNames={{input: classes.languageSelectInput}}
                {...form.getInputProps("language")}
                onOptionSubmit={selectLanguage}
                style={{
                  width: `${(form.values.language?.length || 6) + 6}ch`,
                }}
                disabled={disabled}
              />
            )}
          </div>
          <div style={{flex: 1, display: "flex", justifyContent: "center"}}>
            {problemSelect && (
              <Select
                data={languages}
                aria-label="Язык программирования"
                allowDeselect={false}
                variant="subtle"
                classNames={{input: classes.languageSelectInput}}
                {...form.getInputProps("language")}
                onOptionSubmit={selectLanguage}
                style={{
                  width: `${(form.values.language?.length || 6) + 6}ch`,
                }}
                disabled={disabled}
              />
            )}
          </div>
          <div style={{flex: 1, display: "flex", justifyContent: "flex-end"}}>
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
                accept=".py,.cpp,.go,.txt"
                disabled={disabled}
              />
            </Button>
          </div>
        </Group>

        <div className={classes.dropZone}>
          {isDragging && (
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
          )}
          {!isDragging && file && (
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
          )}
          {!isDragging && !file && (
            <div className={classes.editorContainer}>
              {mounted && (
                <TypedEditor
                  value={form.values.code}
                  onValueChange={setCode}
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
          loading={isPending}
          disabled={disabled}
          color={APP_COLORS.submissions}
        >
          Отправить решение
        </Button>
      </Stack>
    </form>
  );
};

export {CreateSubmissionForm};
