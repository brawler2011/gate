"use client";

import {ActionIcon, Button, Center, Group, Select, Stack, Text} from "@mantine/core";
import {useForm} from "@mantine/form";
import {IconPaperclip, IconTrash} from "@tabler/icons-react";
import React, {useRef, useState, useTransition, type ReactNode} from "react";
import Editor from "react-simple-code-editor";

import {detectSubmissionLanguage} from "@/lib/detectSubmissionLanguage";
import {highlightCode, useGrammar} from "@/lib/highlightCode";
import {languageForExtension, type SupportedLanguage} from "@/lib/languages";
import {APP_COLORS} from "@/lib/theme/colors";

import classes from "./CreateSubmissionForm.module.css";

import type CodeEditor from "react-simple-code-editor";

import "./vsc-dark-plus.css";

// Dynamic import to avoid hydration mismatch (autoCapitalize="off" vs "none")
type CodeEditorProps = React.ComponentProps<typeof CodeEditor>;

const TypedEditor = Editor as React.ComponentType<CodeEditorProps>;

const AUTO = "auto";

type Props = {
  /** Configurations the contestant may choose, in catalog order. */
  languages: SupportedLanguage[];
  /** Receives the source and the identifier of the chosen configuration. */
  onSubmit: (submission: FormData, language: string) => Promise<number | null>;
  problemSelect?: React.ReactNode;
  large?: boolean;
  disabled?: boolean;
};

const CreateSubmissionForm = ({
  languages,
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
      language: AUTO,
    },
    validate: {
      code: (value) => (!value && !file ? "Code or file is required" : null),
      language: (value) =>
        languages.some((language) => language.id === value)
          ? null : "Не удалось определить язык. Выберите его вручную.",
    },
  });
  const options = [
    {value: AUTO, label: "Автоопределение"},
    ...languages.map((language) => ({value: language.id, label: language.name})),
  ];
  const selected = languages.find((language) => language.id === form.values.language);
  const grammar = useGrammar(selected?.syntax);
  const selectWidth = `${(options.find((option) => option.value === form.values.language)?.label.length ?? 6) + 6}ch`;
  const accept = [...new Set(languages.flatMap((language) => language.extensions)), "txt"].map((extension) => "." + extension);

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
          // The detected language suggests the first allowed configuration for its extension.
          const extension = detectSubmissionLanguage(source);
          setFieldValue("language", (extension && languageForExtension(languages, extension)?.id) || AUTO);
        }
      } catch {
        if (!cancelled) {
          setFieldValue("language", AUTO);
        }
      }
    }, 200);
    return () => {
      cancelled = true;
      clearTimeout(timer);
    };
  }, [automaticLanguage, file, form.values.code, form.values.language, languages, setFieldValue]);

  const setCode = (code: string) => {
    form.setFieldValue("code", code);
    if (automaticLanguage) {
      form.setFieldValue("language", AUTO);
    }
  };

  const selectLanguage = (language: string | null) => {
    if (language) {
      setAutomaticLanguage(language === AUTO);
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
    if (droppedFile && accept.some((extension) => droppedFile.name.toLowerCase().endsWith(extension))) {
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
                data={options}
                aria-label="Язык программирования"
                allowDeselect={false}
                variant="subtle"
                classNames={{input: classes.languageSelectInput}}
                {...form.getInputProps("language")}
                onOptionSubmit={selectLanguage}
                style={{width: selectWidth}}
                disabled={disabled}
              />
            )}
          </div>
          <div style={{flex: 1, display: "flex", justifyContent: "center"}}>
            {problemSelect && (
              <Select
                data={options}
                aria-label="Язык программирования"
                allowDeselect={false}
                variant="subtle"
                classNames={{input: classes.languageSelectInput}}
                {...form.getInputProps("language")}
                onOptionSubmit={selectLanguage}
                style={{width: selectWidth}}
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
                accept={accept.join(",")}
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
                  Поддерживаются {accept.join(", ")}
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
                    highlightCode(code, grammar)
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
