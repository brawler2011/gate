"use client";

import {ActionIcon, Button, Center, Group, Select, Stack, Text} from "@mantine/core";
import {useForm} from "@mantine/form";
import {useLocalStorage} from "@mantine/hooks";
import {IconPaperclip, IconTrash} from "@tabler/icons-react";
import React, {useRef, useState, useTransition, type ReactNode} from "react";
import Editor from "react-simple-code-editor";

import {highlightCode, useGrammar} from "@/lib/highlightCode";
import {languageForExtension, type SupportedLanguage} from "@/lib/languages";
import {APP_COLORS} from "@/lib/theme/colors";

import classes from "./CreateSubmissionForm.module.css";

import type CodeEditor from "react-simple-code-editor";

import "./vsc-dark-plus.css";

// Dynamic import to avoid hydration mismatch (autoCapitalize="off" vs "none")
type CodeEditorProps = React.ComponentProps<typeof CodeEditor>;

const TypedEditor = Editor as React.ComponentType<CodeEditorProps>;

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
  const [savedLanguage, setSavedLanguage] = useLocalStorage<string>({
    key: "submission-language",
    defaultValue: "",
    getInitialValueInEffect: false,
    sync: false,
  });
  const selected = languages.find((language) => language.id === savedLanguage) ?? languages[0];
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
    },
    validate: {
      code: (value) => (!value && !file ? "Code or file is required" : null),
    },
  });
  const options = languages.map((language) => ({value: language.id, label: language.name}));
  const grammar = useGrammar(selected?.syntax);
  const selectWidth = `${(selected?.name.length ?? 6) + 6}ch`;
  const accept = [...new Set(languages.flatMap((language) => language.extensions)), "txt"].map((extension) => "." + extension);

  React.useEffect(() => {
    if (selected && selected.id !== savedLanguage) {
      setSavedLanguage(selected.id);
    }
  }, [selected, savedLanguage, setSavedLanguage]);

  const setCode = (code: string) => {
    form.setFieldValue("code", code);
  };

  const selectLanguage = (language: string | null) => {
    if (language) {
      setSavedLanguage(language);
    }
  };

  const attachFile = (file: File) => {
    const extension = file.name.includes(".") ? file.name.split(".").pop()! : "";
    const language = languageForExtension(languages, extension, savedLanguage) ?? selected;
    if (language) {
      setSavedLanguage(language.id);
    }
    setFile(file);
    setCode("");
  };

  const [isPending, startTransition] = useTransition();

  const onSubmitHandler = (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!selected) {
      return;
    }
    form.onSubmit((values) => {
      startTransition(async () => {
        try {
          const formData = new FormData();
          if (file) {
            formData.append("submission", file);
          } else {
            formData.append("submission", values.code);
          }
          const data = await onSubmit(formData, selected.id);
          if (data) {
            form.reset();
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
      attachFile(selectedFile);
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
    if (droppedFile) {
      attachFile(droppedFile);
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
                value={selected?.id ?? null}
                onChange={selectLanguage}
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
                value={selected?.id ?? null}
                onChange={selectLanguage}
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
          disabled={disabled || !selected}
          color={APP_COLORS.submissions}
        >
          Отправить решение
        </Button>
      </Stack>
    </form>
  );
};

export {CreateSubmissionForm};
