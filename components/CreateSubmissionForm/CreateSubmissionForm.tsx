"use client";

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
import React, { useRef, useState } from "react";
import { IconTrash, IconUpload } from "@tabler/icons-react";
import { useMutation } from "@tanstack/react-query";
import classes from "./styles.module.css";
import { APP_COLORS } from "@/lib/theme/colors";
import { highlightCode } from "@/lib/highlightCode";
import dynamic from "next/dynamic";
import "./vsc-dark-plus.css";

// Dynamic import to avoid hydration mismatch (autoCapitalize="off" vs "none")
const Editor = dynamic(
    () => import("react-simple-code-editor").then((mod) => mod.default),
    { ssr: false }
);

const languages = ["python", "cpp", "golang"];

type Props = {
    onSubmit: (submission: FormData, language: string) => Promise<number | null>;
    problemSelect?: React.ReactNode;
    large?: boolean;
    disabled?: boolean;
};

const CreateSubmissionForm = ({ onSubmit, problemSelect, large = false, disabled = false }: Props) => {
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
            language: (value) => (languages.includes(value) ? null : "Invalid language"),
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

    const handleKeyDown = (event: React.KeyboardEvent<HTMLTextAreaElement>) => {
        if (event.key === "Tab") {
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
        <form
            onSubmit={form.onSubmit((values) => mutation.mutate(values))}
        >
            <Stack
                className={`${classes.code} ${large ? classes.codeLarge : ''}`}
                onDrop={handleDrop}
                onDragOver={handleDragOver}
                onDragLeave={handleDragLeave}
                data-dragging={isDragging}
            >
                <Group justify="space-between">
                    {problemSelect && problemSelect}
                    <Select
                        data={languages}
                        allowDeselect={false}
                        {...form.getInputProps("language")}
                        style={{ width: "200px" }}
                        disabled={disabled}
                    />
                    <Button
                        component="label"
                        variant="light"
                        leftSection={<IconUpload size={16} />}
                        classNames={{
                            label: classes.pinFileLabel,
                            section: classes.pinFileSection,
                            root: classes.pinFileRoot,
                        }}
                        disabled={disabled}
                    >
                        Прикрепить файл
                        <input
                            type="file"
                            hidden
                            ref={fileInputRef}
                            onChange={handleFileSelect}
                            accept=".py,.cpp,.go,.txt"
                            disabled={disabled}
                        />
                    </Button>
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
                                <Editor
                                    value={form.values.code}
                                    onValueChange={(code: string) => form.setFieldValue("code", code)}
                                    highlight={(code: string) => highlightCode(code, form.values.language)}
                                    padding={10}
                                    placeholder="Введите ваше решение здесь, перетащите файл или текст..."
                                    className={classes.codeEditor}
                                    disabled={disabled}
                                    textareaId="code-editor-textarea"
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
