"use client";

import { SectionPaper } from "@/components/workshop/SectionPaper";
import {
  getWorkshopProblemLimits,
  getWorkshopProblemStatement,
  updateWorkshopProblemStatement,
} from "@/lib/actions";
import {
  Box,
  Button,
  Group,
  Modal,
  Select,
  Stack,
  Text,
  TextInput,
  Textarea,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import "katex/dist/katex.min.css";
import { useDeferredValue, useEffect, useState, useTransition } from "react";
import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import "../problems/Problem.css";
import classes from "./WorkshopStatementTab.module.css";

type StatementData = {
  title: string;
  legend: string;
  input_format: string;
  output_format: string;
  notes: string;
  interaction: string;
  scoring: string;
};

type PreviewMeta = {
  problem_type?: string;
  max_score?: number | null;
  time_limit_ms?: number;
  memory_limit_mb?: number;
};

type LoadedPreviewMeta = {
  problem_type: string;
  max_score: number | null;
  time_limit_ms: number;
  memory_limit_mb: number;
};

type Props = {
  problemId: string;
};

function prettifyTimeLimit(timeLimit: number) {
  if (timeLimit % 1000 === 0) {
    return `${timeLimit / 1000} сек`;
  }

  return `${timeLimit} мс`;
}

function prettifyMemoryLimit(memoryLimit: number) {
  if (memoryLimit % 1000 === 0) {
    return `${memoryLimit / 1000} ГБ`;
  }

  return `${memoryLimit} МБ`;
}

function hasPreviewMeta(meta: PreviewMeta | null): meta is LoadedPreviewMeta {
  return (
    meta?.problem_type !== undefined &&
    meta.max_score !== undefined &&
    meta.time_limit_ms !== undefined &&
    meta.memory_limit_mb !== undefined
  );
}

function MarkdownBlock({ value }: { value: string }) {
  return (
    <div className="content">
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[rehypeKatex]}
      >
        {value}
      </ReactMarkdown>
    </div>
  );
}

function PreviewSection({ title, value }: { title: string; value: string }) {
  if (!value.trim()) return null;

  return (
    <Stack gap="xs">
      <Title order={3} className={classes.sectionTitle}>
        {title}
      </Title>
      <MarkdownBlock value={value} />
    </Stack>
  );
}

function WorkshopStatementPreview({
  statement,
  previewMeta,
}: {
  statement: StatementData;
  previewMeta: LoadedPreviewMeta;
}) {
  const hasContent = [
    statement.legend,
    statement.input_format,
    statement.output_format,
    statement.notes,
    statement.scoring,
  ].some((value) => value.trim());

  return (
    <Stack className="container" gap="md">
      <Stack align="center" gap={0} w="fit-content" mx="auto" mb="sm">
        <Title
          order={2}
        >{`A. ${statement.title.trim() || "Без названия"}`}</Title>
        <Stack align="center" gap={0}>
          <Text>
            ограничение по времени:{" "}
            {prettifyTimeLimit(previewMeta.time_limit_ms)}
          </Text>
          <Text>
            ограничение по памяти:{" "}
            {prettifyMemoryLimit(previewMeta.memory_limit_mb)}
          </Text>
          {previewMeta.problem_type === "scoring" &&
          previewMeta.max_score !== null ? (
            <Text>максимальный балл: {previewMeta.max_score}</Text>
          ) : null}
        </Stack>
      </Stack>

      {hasContent ? (
        <>
          {statement.legend.trim() ? (
            <MarkdownBlock value={statement.legend} />
          ) : null}
          <PreviewSection
            title="Входные данные"
            value={statement.input_format}
          />
          <PreviewSection
            title="Выходные данные"
            value={statement.output_format}
          />
          <PreviewSection title="Система оценки" value={statement.scoring} />
          <PreviewSection title="Примечание" value={statement.notes} />
        </>
      ) : (
        <Text c="dimmed" ta="center">
          Начни заполнять поля слева, и здесь появится preview условия.
        </Text>
      )}
    </Stack>
  );
}

export function WorkshopStatementTab({ problemId }: Props) {
  const [statement, setStatement] = useState<StatementData | null>(null);
  const [previewMeta, setPreviewMeta] = useState<PreviewMeta | null>(null);
  const [languages, setLanguages] = useState<string[]>(["en"]);
  const [activeLang, setActiveLang] = useState<string>("en");
  const [isDirty, setIsDirty] = useState(false);
  const [isLoading, startLoading] = useTransition();
  const [isSaving, startSaving] = useTransition();
  const deferredStatement = useDeferredValue(statement);

  // States for custom Mantine modals
  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
  const [pendingLang, setPendingLang] = useState<string | null>(null);
  const [pendingAction, setPendingAction] = useState<"switch" | "add" | null>(null);
  const [isAddLangOpen, setIsAddLangOpen] = useState(false);
  const [newLangCode, setNewLangCode] = useState("");
  const [newLangError, setNewLangError] = useState("");
  const [isAddingServerLang, setIsAddingServerLang] = useState(false);

  const loadStatement = (lang: string) => {
    startLoading(async () => {
      const [[limitsError, limits], [statementError, statementData]] =
        await Promise.all([
          getWorkshopProblemLimits(problemId),
          getWorkshopProblemStatement(problemId, lang),
        ]);

      if (limitsError || !limits) {
        notifications.show({
          title: "Ошибка загрузки лимитов",
          message:
            limitsError?.message ?? "Не удалось загрузить лимиты для preview",
          color: "red",
        });
        return;
      }

      if (statementError || !statementData) {
        notifications.show({
          title: "Ошибка загрузки условия",
          message: statementError?.message ?? "Не удалось загрузить statement",
          color: "red",
        });
        return;
      }

      setPreviewMeta({
        problem_type: limits.problem_type,
        max_score: limits.max_score ?? null,
        time_limit_ms: limits.time_limit_ms,
        memory_limit_mb: limits.memory_limit_mb,
      });
      setStatement({
        title: statementData.title ?? "",
        legend: statementData.legend ?? "",
        input_format: statementData.input_format ?? "",
        output_format: statementData.output_format ?? "",
        notes: statementData.notes ?? "",
        interaction: statementData.interaction ?? "",
        scoring: statementData.scoring ?? "",
      });
      setLanguages(statementData.languages ?? [lang]);
      setActiveLang(statementData.current_lang ?? lang);
      setIsDirty(false);
    });
  };

  useEffect(() => {
    loadStatement(activeLang);
  }, [problemId]);

  useEffect(() => {
    const handleBeforeUnload = (e: BeforeUnloadEvent) => {
      if (isDirty) {
        e.preventDefault();
        e.returnValue = "У вас есть несохраненные изменения.";
        return e.returnValue;
      }
    };
    window.addEventListener("beforeunload", handleBeforeUnload);
    return () => {
      window.removeEventListener("beforeunload", handleBeforeUnload);
    };
  }, [isDirty]);

  const handleLangChange = (newValue: string | null) => {
    if (!newValue) return;
    if (newValue === "add_new_lang") {
      if (isDirty) {
        setPendingAction("add");
        setIsConfirmOpen(true);
      } else {
        handleAddLanguageClick();
      }
    } else {
      if (isDirty) {
        setPendingLang(newValue);
        setPendingAction("switch");
        setIsConfirmOpen(true);
      } else {
        loadStatement(newValue);
      }
    }
  };

  const confirmDiscardChanges = () => {
    if (pendingAction === "switch" && pendingLang) {
      loadStatement(pendingLang);
    } else if (pendingAction === "add") {
      handleAddLanguageClick();
    }
    setIsConfirmOpen(false);
    setPendingLang(null);
    setPendingAction(null);
  };

  const handleAddLanguageClick = () => {
    setNewLangCode("");
    setNewLangError("");
    setIsAddLangOpen(true);
  };

  const submitAddLanguage = async () => {
    const cleanLang = newLangCode.trim().toLowerCase();
    if (!cleanLang) {
      setNewLangError("Код языка не может быть пустым");
      return;
    }
    if (cleanLang.length !== 2 || !/^[a-zA-Z]{2}$/.test(cleanLang)) {
      setNewLangError("Код языка должен состоять ровно из 2 букв на английском языке (например: ru, en)");
      return;
    }
    if (languages.includes(cleanLang)) {
      setNewLangError("Этот язык уже добавлен");
      return;
    }

    setIsAddingServerLang(true);
    const [saveError] = await updateWorkshopProblemStatement(problemId, {
      title: statement?.title ?? "",
      legend: "",
      input_format: "",
      output_format: "",
      notes: "",
      interaction: "",
      scoring: "",
    }, cleanLang);

    if (saveError) {
      setNewLangError(saveError.message ?? "Не удалось создать файл на сервере");
      setIsAddingServerLang(false);
      return;
    }

    setIsAddingServerLang(false);
    setIsAddLangOpen(false);

    notifications.show({
      title: "Создано",
      message: `Создано новое условие для языка ${cleanLang.toUpperCase()}`,
      color: "green",
    });

    loadStatement(cleanLang);
  };

  const patchStatement = (patch: Partial<StatementData>) => {
    setStatement((prev) => ({
      title: "",
      legend: "",
      input_format: "",
      output_format: "",
      notes: "",
      interaction: "",
      scoring: "",
      ...prev,
      ...patch,
    }));
    setIsDirty(true);
  };

  const handleSave = () => {
    startSaving(async () => {
      if (!statement) return;

      const [saveError] = await updateWorkshopProblemStatement(problemId, {
        title: statement.title,
        legend: statement.legend,
        input_format: statement.input_format,
        output_format: statement.output_format,
        notes: statement.notes,
        interaction: statement.interaction,
        scoring: statement.scoring,
      }, activeLang);

      if (saveError) {
        notifications.show({
          title: "Ошибка сохранения",
          message: saveError.message ?? "Не удалось сохранить условие",
          color: "red",
        });
        return;
      }

      setIsDirty(false);
      notifications.show({
        title: "Сохранено",
        message: "Условие задачи обновлено",
        color: "green",
      });

      // Reload to update list of languages
      loadStatement(activeLang);
    });
  };

  return (
    <Box className={classes.outerRoot}>
      <Box className={classes.topBar}>
        <Group justify="flex-end" align="center" px="lg" py="sm" gap="lg">
          {isDirty && (
            <Text size="xs" c="orange" fw={500}>
              Несохраненные изменения
            </Text>
          )}
          <Button
            size="sm"
            disabled={!isDirty}
            loading={isSaving}
            onClick={handleSave}
          >
            Сохранить
          </Button>
          <Group gap="xs" align="center">
            <Text size="sm" fw={500} c="dimmed">
              Язык:
            </Text>
            <Select
              value={activeLang}
              onChange={handleLangChange}
              data={[
                ...languages.map((l) => ({
                  label: l.toUpperCase(),
                  value: l,
                })),
                { label: "+ Добавить язык...", value: "add_new_lang" },
              ]}
              allowDeselect={false}
              w={180}
              disabled={isLoading}
            />
          </Group>
        </Group>
      </Box>

      <Box className={classes.root}>
        <Box className={classes.editorPane}>
          <Box p="lg">
            <Stack gap="lg" maw={900} mx="auto">
              <SectionPaper>
                {isLoading ? (
                  <Text c="dimmed" size="sm">
                    Загрузка...
                  </Text>
                ) : (
                  <Stack gap="md">
                    {!statement ? null : (
                      <>
                      <TextInput
                        label="Заголовок"
                        value={statement.title}
                        onChange={(e) =>
                          patchStatement({ title: e.currentTarget.value })
                        }
                      />

                      <Textarea
                        label="Легенда"
                        value={statement.legend}
                        onChange={(e) =>
                          patchStatement({ legend: e.currentTarget.value })
                        }
                        minRows={6}
                        autosize
                      />

                      <Textarea
                        label="Формат входных данных"
                        value={statement.input_format}
                        onChange={(e) =>
                          patchStatement({
                            input_format: e.currentTarget.value,
                          })
                        }
                        minRows={4}
                        autosize
                      />

                      <Textarea
                        label="Формат выходных данных"
                        value={statement.output_format}
                        onChange={(e) =>
                          patchStatement({
                            output_format: e.currentTarget.value,
                          })
                        }
                        minRows={4}
                        autosize
                      />

                      <Textarea
                        label="Примечания"
                        value={statement.notes}
                        onChange={(e) =>
                          patchStatement({ notes: e.currentTarget.value })
                        }
                        minRows={3}
                        autosize
                      />

                      <Textarea
                        label="Интерактивное взаимодействие"
                        value={statement.interaction}
                        onChange={(e) =>
                          patchStatement({ interaction: e.currentTarget.value })
                        }
                        minRows={3}
                        autosize
                      />

                      <Textarea
                        label="Система оценки"
                        value={statement.scoring}
                        onChange={(e) =>
                          patchStatement({ scoring: e.currentTarget.value })
                        }
                        minRows={3}
                        autosize
                      />

                    </>
                  )}
                </Stack>
              )}
            </SectionPaper>
          </Stack>
        </Box>
      </Box>

      <Box className={classes.previewPane} visibleFrom="md">
        <Box p="lg">
          <Stack gap="lg" maw={900} mx="auto">
            {isLoading || !deferredStatement || !hasPreviewMeta(previewMeta) ? (
              <Text c="dimmed" size="sm">
                Загрузка...
              </Text>
            ) : (
              <WorkshopStatementPreview
                statement={deferredStatement}
                previewMeta={previewMeta}
              />
            )}
          </Stack>
        </Box>
      </Box>
    </Box>

      {/* Confirmation Modal for Unsaved Changes */}
      <Modal
        opened={isConfirmOpen}
        onClose={() => setIsConfirmOpen(false)}
        title="Несохраненные изменения"
        centered
      >
        <Stack gap="md">
          <Text size="sm">
            У вас есть несохраненные изменения. При переходе на другой язык или добавлении нового все несохраненные изменения будут потеряны. Вы уверены, что хотите продолжить?
          </Text>
          <Group justify="flex-end" gap="xs">
            <Button variant="subtle" color="gray" onClick={() => setIsConfirmOpen(false)}>
              Отмена
            </Button>
            <Button color="red" onClick={confirmDiscardChanges}>
              Продолжить без сохранения
            </Button>
          </Group>
        </Stack>
      </Modal>

      {/* Modal for Adding New Language */}
      <Modal
        opened={isAddLangOpen}
        onClose={() => !isAddingServerLang && setIsAddLangOpen(false)}
        title="Добавить язык условия"
        centered
      >
        <Stack gap="md">
          <Text size="sm" c="dimmed">
            Введите двухсимвольный ISO-код языка на английском (например, ru, en, de) или выберите из быстрых вариантов ниже.
          </Text>
          
          <TextInput
            label="Код языка"
            placeholder="например: ru, en"
            value={newLangCode}
            onChange={(e) => {
              setNewLangCode(e.currentTarget.value);
              setNewLangError("");
            }}
            error={newLangError}
            maxLength={2}
            autoFocus
            disabled={isAddingServerLang}
          />

          <Group gap="xs">
            {["ru", "en", "de"].map((code) => (
              <Button
                key={code}
                variant="light"
                size="xs"
                disabled={isAddingServerLang}
                onClick={() => {
                  setNewLangCode(code);
                  setNewLangError("");
                }}
              >
                {code.toUpperCase()}
              </Button>
            ))}
          </Group>

          <Group justify="flex-end" gap="xs" mt="sm">
            <Button
              variant="subtle"
              color="gray"
              onClick={() => setIsAddLangOpen(false)}
              disabled={isAddingServerLang}
            >
              Отмена
            </Button>
            <Button
              onClick={submitAddLanguage}
              loading={isAddingServerLang}
            >
              Добавить
            </Button>
          </Group>
        </Stack>
      </Modal>
    </Box>
  );
}
