import {
  ActionIcon,
  Box,
  Button,
  Flex,
  Group,
  Modal,
  Paper,
  Select,
  Stack,
  Text,
  TextInput,
  Textarea,
  Title,
  Tooltip,
} from "@mantine/core";
import {useClipboard} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {IconCheck, IconCopy} from "@tabler/icons-react";
import {useDeferredValue, useEffect, useState, useTransition, type ReactNode} from "react";
import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

import "katex/dist/katex.min.css";

import {SectionPaper} from "@/components/workshop/SectionPaper";
import {api} from "@/lib/api";

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

const prettifyTimeLimit = (timeLimit: number) => {
  if (timeLimit % 1000 === 0) {
    return `${timeLimit / 1000} сек`;
  }

  return `${timeLimit} мс`;
};

const prettifyMemoryLimit = (memoryLimit: number) => {
  if (memoryLimit % 1000 === 0) {
    return `${memoryLimit / 1000} ГБ`;
  }

  return `${memoryLimit} МБ`;
};

const hasPreviewMeta = (meta: PreviewMeta | null): meta is LoadedPreviewMeta => {
  return (
    meta?.problem_type !== undefined &&
    meta.max_score !== undefined &&
    meta.time_limit_ms !== undefined &&
    meta.memory_limit_mb !== undefined
  );
};

const renderSafeImage = (problemId?: string) => {
  const SafeImage = ({
    node: _node,
    src,
    alt,
    ...props
  }: React.ImgHTMLAttributes<HTMLImageElement> & { node?: unknown }) => {
    if (!src) {
      return null;
    }

    let width: string | undefined;
    let height: string | undefined;
    let isCentered = false;
    let cleanSrc = src;

    const hashIndex = src.indexOf("#");
    if (hashIndex !== -1) {
      cleanSrc = src.slice(0, hashIndex);
      const hashParts = src.slice(hashIndex + 1).split("#");

      for (const part of hashParts) {
        if (part === "center" || part === "middle") {
          isCentered = true;
        } else if (/^\d+(x\d*)?$/.test(part)) {
          if (part.includes("x")) {
            const [w, h] = part.split("x");
            if (w) {
              width = `${w}px`;
            }
            if (h) {
              height = `${h}px`;
            }
          } else {
            width = `${part}px`;
          }
        } else if (/^(\d+(x\d*)?)-center$/.test(part) || /^center-(\d+(x\d*)?)$/.test(part)) {
          isCentered = true;
          const num = part.replace(/-?center-?/, "");
          if (num.includes("x")) {
            const [w, h] = num.split("x");
            if (w) {
              width = `${w}px`;
            }
            if (h) {
              height = `${h}px`;
            }
          } else {
            width = `${num}px`;
          }
        }
      }
    }

    if (
      problemId &&
      !cleanSrc.startsWith("http://") &&
      !cleanSrc.startsWith("https://") &&
      !cleanSrc.startsWith("data:") &&
      !cleanSrc.startsWith("/api/")
    ) {
      const filename = cleanSrc.replace(/^\.\//, "").replace(/^media\//, "");
      cleanSrc = `/api/problems/${problemId}/media/${filename}`;
    }

    return (
      // eslint-disable-next-line @next/next/no-img-element
      <img
        src={cleanSrc}
        alt={alt || ""}
        style={{
          maxWidth: "100%",
          width: width,
          height: height || "auto",
          display: isCentered ? "block" : undefined,
          margin: isCentered ? "0 auto" : undefined,
        }}
        {...props}
      />
    );
  };
  return SafeImage;
};

const MarkdownBlock = ({value, problemId}: { value: string; problemId?: string }) => {
  return (
    <div className="content">
      <ReactMarkdown
        remarkPlugins={[remarkGfm, remarkMath]}
        rehypePlugins={[rehypeKatex]}
        components={{
          img: renderSafeImage(problemId),
        }}
      >
        {value}
      </ReactMarkdown>
    </div>
  );
};

const PreviewSection = ({title, value, problemId}: { title: string; value: string; problemId?: string }) => {
  if (!value.trim()) {
    return null;
  }

  return (
    <Stack gap="xs">
      <Title order={3} className={classes.sectionTitle}>
        {title}
      </Title>
      <MarkdownBlock value={value} problemId={problemId} />
    </Stack>
  );
};

const CopyableSection = ({label, value}: { label: string; value: string }) => {
  const clipboard = useClipboard({timeout: 2000});
  return (
    <Stack gap="xs" style={{flex: "1 1 300px", minWidth: 0}}>
      <Group justify="space-between" align="center" h={28}>
        <Text fw={600} size="sm">{label}</Text>
        <Tooltip label={clipboard.copied ? "Скопировано!" : "Скопировать"} position="top" withArrow>
          <ActionIcon
            variant="subtle"
            color={clipboard.copied ? "green" : "gray"}
            onClick={() => clipboard.copy(value)}
            size="sm"
          >
            {clipboard.copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
          </ActionIcon>
        </Tooltip>
      </Group>
      <Box
        component="pre"
        p="xs"
        bg="light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-6))"
        style={{
          border: "1px solid light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-5))",
          borderRadius: "var(--mantine-radius-sm)",
          overflowX: "auto",
          fontFamily: "var(--mantine-font-monospace)",
          fontSize: "var(--mantine-font-size-sm)",
          whiteSpace: "pre",
          margin: 0,
        }}
      >
        {value}
      </Box>
    </Stack>
  );
};

const WorkshopStatementPreview = ({
  statement,
  previewMeta,
  samples,
  problemId,
}: {
  statement: StatementData;
  previewMeta: LoadedPreviewMeta;
  samples: Array<{ input: string; output: string }>;
  problemId?: string;
}) => {
  const hasContent = [
    statement.legend,
    statement.input_format,
    statement.output_format,
    statement.notes,
    statement.scoring,
  ].some((value) => value.trim()) || (samples && samples.length > 0);

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
            <MarkdownBlock value={statement.legend} problemId={problemId} />
          ) : null}
          <PreviewSection
            title="Входные данные"
            value={statement.input_format}
            problemId={problemId}
          />
          <PreviewSection
            title="Выходные данные"
            value={statement.output_format}
            problemId={problemId}
          />
          {samples && samples.length > 0 && (
            <Stack gap="xs">
              <Title order={3} className={classes.sectionTitle}>
                Примеры
              </Title>
              <Stack gap="md">
                {samples.map((sample, index) => (
                  <Paper
                    key={index}
                    withBorder
                    p="md"
                    radius="md"
                    bg="light-dark(var(--mantine-color-white), var(--mantine-color-dark-7))"
                  >
                    <Stack gap="xs">
                      <Text fw={700} size="sm" c="dimmed">
                        Пример {index + 1}
                      </Text>
                      <Flex gap="md" wrap="wrap">
                        <CopyableSection label="Входные данные" value={sample.input} />
                        <CopyableSection label="Выходные данные" value={sample.output} />
                      </Flex>
                    </Stack>
                  </Paper>
                ))}
              </Stack>
            </Stack>
          )}
          <PreviewSection title="Система оценки" value={statement.scoring} problemId={problemId} />
          <PreviewSection title="Примечание" value={statement.notes} problemId={problemId} />
        </>
      ) : (
        <Text c="dimmed" ta="center">
          Начни заполнять поля слева, и здесь появится preview условия.
        </Text>
      )}
    </Stack>
  );
};

export const WorkshopStatementTab = ({problemId}: Props): ReactNode => {
  const [statement, setStatement] = useState<StatementData | null>(null);
  const [previewMeta, setPreviewMeta] = useState<PreviewMeta | null>(null);
  const [samples, setSamples] = useState<Array<{ input: string; output: string }>>([]);
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
      const [[limitsError, limits], [statementError, statementData], [problemError, problemData]] =
        await Promise.all([
          api.getProblemLimits({problemId}),
          api.getProblemStatement({problemId, lang}),
          api.getProblem({id: problemId}),
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

      if (!problemError && problemData?.problem) {
        setSamples(problemData.problem.samples || []);
      } else {
        setSamples([]);
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
    if (!newValue) {
      return;
    }
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
    const [saveError] = await api.updateProblemStatement({
      problemId,
      requestBody: {
        title: statement?.title ?? "",
        legend: "",
        input_format: "",
        output_format: "",
        notes: "",
        interaction: "",
        scoring: "",
      },
      lang: cleanLang,
    });

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
      if (!statement) {
        return;
      }

      const [saveError] = await api.updateProblemStatement({
        problemId,
        requestBody: {
          title: statement.title,
          legend: statement.legend,
          input_format: statement.input_format,
          output_format: statement.output_format,
          notes: statement.notes,
          interaction: statement.interaction,
          scoring: statement.scoring,
        },
        lang: activeLang,
      });

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
                {label: "+ Добавить язык...", value: "add_new_lang"},
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
                            patchStatement({title: e.currentTarget.value})
                          }
                        />

                        <Textarea
                          label="Легенда"
                          value={statement.legend}
                          onChange={(e) =>
                            patchStatement({legend: e.currentTarget.value})
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
                            patchStatement({notes: e.currentTarget.value})
                          }
                          minRows={3}
                          autosize
                        />

                        <Textarea
                          label="Интерактивное взаимодействие"
                          value={statement.interaction}
                          onChange={(e) =>
                            patchStatement({interaction: e.currentTarget.value})
                          }
                          minRows={3}
                          autosize
                        />

                        <Textarea
                          label="Система оценки"
                          value={statement.scoring}
                          onChange={(e) =>
                            patchStatement({scoring: e.currentTarget.value})
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
                  samples={samples}
                  problemId={problemId}
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
            У вас есть несохраненные изменения. При переходе на другой язык
            или добавлении нового все несохраненные изменения будут потеряны.
            Вы уверены, что хотите продолжить?
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
            Введите двухсимвольный ISO-код языка на английском (например, ru, en, de)
            или выберите из быстрых вариантов ниже.
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
};
