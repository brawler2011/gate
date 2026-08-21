"use client";

import {
  Alert,
  Badge,
  Box,
  Button,
  Card,
  Divider,
  Grid,
  Group,
  Loader,
  Paper,
  Select,
  Stack,
  Switch,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import {
  IconAlertCircle,
  IconCheck,
  IconCopy,
  IconDeviceFloppy,
  IconFileCode,
  IconInfoCircle,
  IconRefresh,
  IconTrash,
  IconUser,
} from "@tabler/icons-react";
import dynamic from "next/dynamic";
import React, {useCallback, useEffect, useState, useTransition, type ReactNode} from "react";

import {api} from "@/lib/api";
import {LANGUAGE_MAP} from "@/lib/constants";
import {highlightCode} from "@/lib/highlightCode";
import {numberToLetters} from "@/lib/lib";

import classes from "./DraftsClient.module.css";
import "@/components/submissions/vsc-dark-plus.css";

import type {
  ContestDraftModel,
  ContestMemberModel,
  ContestModel,
  ContestProblemListItemModel,
  UserModel,
} from "@/contracts/core/v1";
import type CodeEditor from "react-simple-code-editor";

type CodeEditorProps = React.ComponentProps<typeof CodeEditor>;

const Editor = dynamic(
  () => import("react-simple-code-editor").then((mod) => mod.default),
  {ssr: false},
);

const TypedEditor = Editor as React.ComponentType<CodeEditorProps>;

const languages = [
  {value: "cpp", label: "C++"},
  {value: "python", label: "Python"},
  {value: "golang", label: "Go"},
];

const languageIdToKey: Record<number, string> = {
  10: "golang",
  20: "cpp",
  30: "python",
};

const languageIdToLabel: Record<number, string> = {
  10: "Go",
  20: "C++",
  30: "Python",
};

type Props = {
  contest: ContestModel;
  problems: ContestProblemListItemModel[];
  user: UserModel | null;
  isManager: boolean;
  isContestEnded: boolean;
  initialDrafts: ContestDraftModel[];
  members?: ContestMemberModel[];
};

export const DraftsClient = ({
  contest,
  problems,
  user: _user,
  isManager,
  isContestEnded,
  initialDrafts,
  members = [],
}: Props): ReactNode => {
  const [mounted, setMounted] = useState(false);
  const [selectedProblemId, setSelectedProblemId] = useState<string | null>(
    problems.length > 0 ? problems[0].problem_id : null,
  );
  const [language, setLanguage] = useState<string>("cpp");
  const [code, setCode] = useState<string>("");
  const [drafts, setDrafts] = useState<ContestDraftModel[]>(initialDrafts);
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);
  const [filterOnlyProblem, setFilterOnlyProblem] = useState<boolean>(true);
  const [loading, setLoading] = useState<boolean>(false);
  const [statusMessage, setStatusMessage] = useState<{type: "success" | "error"; text: string} | null>(null);
  const [copiedDraftId, setCopiedDraftId] = useState<string | null>(null);
  const [editorCopied, setEditorCopied] = useState<boolean>(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const [isPending, startTransition] = useTransition();

  useEffect(() => {
    setMounted(true);
  }, []);

  const fetchDrafts = useCallback(async (userId?: string | null, problemId?: string | null) => {
    setLoading(true);
    const [error, data] = await api.listContestDrafts({
      orgLogin: contest.organization_login,
      contestLogin: contest.login,
      userId: userId || undefined,
      problemId: problemId || undefined,
      page: 1,
      pageSize: 50,
    });
    setLoading(false);
    if (!error && data?.drafts) {
      setDrafts(data.drafts);
    }
  }, [contest.organization_login, contest.login]);

  const handleSaveDraft = async () => {
    if (!selectedProblemId) {
      setStatusMessage({type: "error", text: "Выберите задачу"});
      return;
    }
    if (!code.trim()) {
      setStatusMessage({type: "error", text: "Код черновика не может быть пустым"});
      return;
    }
    if (code.length > 64 * 1024) {
      setStatusMessage({type: "error", text: "Превышен максимальный размер кода (64 КБ)"});
      return;
    }

    const languageCode = LANGUAGE_MAP[language];
    if (!languageCode) {
      setStatusMessage({type: "error", text: "Некорректный язык программирования"});
      return;
    }

    startTransition(async () => {
      setStatusMessage(null);
      const [error, response] = await api.createContestDraft({
        orgLogin: contest.organization_login,
        contestLogin: contest.login,
        requestBody: {
          problem_id: selectedProblemId,
          language: languageCode,
          code,
        },
      });

      if (error) {
        setStatusMessage({type: "error", text: error.message || "Ошибка при сохранении черновика"});
      } else if (response?.id) {
        setStatusMessage({type: "success", text: "Черновик успешно сохранен!"});
        await fetchDrafts(
          isManager ? selectedUserId : undefined,
          filterOnlyProblem ? selectedProblemId : undefined,
        );
      }
    });
  };

  const handleDeleteDraft = async (draftId: string) => {
    setDeletingId(draftId);
    const [error] = await api.deleteContestDraft({
      orgLogin: contest.organization_login,
      contestLogin: contest.login,
      draftId,
    });
    setDeletingId(null);
    if (!error) {
      setDrafts((prev) => prev.filter((d) => d.id !== draftId));
      setStatusMessage({type: "success", text: "Черновик удален"});
    } else {
      setStatusMessage({type: "error", text: error.message || "Не удалось удалить черновик"});
    }
  };

  const handleLoadDraft = (draft: ContestDraftModel) => {
    setCode(draft.code);
    setSelectedProblemId(draft.problem_id);
    const langKey = languageIdToKey[draft.language];
    if (langKey) {
      setLanguage(langKey);
    }
    setStatusMessage({type: "success", text: "Черновик загружен в редактор"});
  };

  const handleCopyCode = async (textToCopy: string, draftId?: string) => {
    try {
      await navigator.clipboard.writeText(textToCopy);
      if (draftId) {
        setCopiedDraftId(draftId);
        setTimeout(() => setCopiedDraftId(null), 2000);
      } else {
        setEditorCopied(true);
        setTimeout(() => setEditorCopied(false), 2000);
      }
    } catch {
      setStatusMessage({type: "error", text: "Не удалось скопировать код в буфер обмена"});
    }
  };

  const problemOptions = problems.map((p) => ({
    value: p.problem_id,
    label: `${numberToLetters(p.position)}. ${p.title}`,
  }));

  const userOptions = [
    {value: "", label: "Все участники"},
    ...members.map((m) => ({
      value: m.contest_id,
      label: m.role || m.contest_role || "Участник",
    })),
  ];

  const filteredDrafts = drafts.filter((d) => {
    if (filterOnlyProblem && selectedProblemId && d.problem_id !== selectedProblemId) {
      return false;
    }
    return true;
  });

  const isSaveDisabled =
    (isContestEnded && !isManager) ||
    !code.trim() ||
    !selectedProblemId ||
    isPending;

  return (
    <Stack gap="lg">
      <Box>
        <Group justify="space-between" align="flex-start" wrap="wrap">
          <div>
            <Title order={2}>Черновики решений</Title>
            <Text c="dimmed" size="sm">
              Сохраняйте рабочие версии кода задач. Черновики доступны только вам и жюри контеста.
            </Text>
          </div>
          {isContestEnded && (
            <Badge color="orange" size="lg" variant="light">
              Контест завершен (только чтение)
            </Badge>
          )}
        </Group>
      </Box>

      {statusMessage && (
        <Alert
          icon={statusMessage.type === "success" ? <IconCheck size="1.1rem" /> : <IconAlertCircle size="1.1rem" />}
          color={statusMessage.type === "success" ? "green" : "red"}
          withCloseButton
          onClose={() => setStatusMessage(null)}
        >
          {statusMessage.text}
        </Alert>
      )}

      {isManager && (
        <Paper withBorder p="md" radius="md">
          <Group justify="space-between" wrap="wrap">
            <Group gap="sm">
              <IconUser size="1.2rem" />
              <Text fw={600} size="sm">Фильтр жюри:</Text>
              <Select
                placeholder="Выберите участника"
                data={userOptions}
                value={selectedUserId || ""}
                onChange={(val) => {
                  const uid = val || null;
                  setSelectedUserId(uid);
                  fetchDrafts(uid, filterOnlyProblem ? selectedProblemId : undefined);
                }}
                clearable
                style={{minWidth: 220}}
              />
            </Group>
            <Button
              variant="subtle"
              size="xs"
              leftSection={<IconRefresh size="1rem" />}
              onClick={() => fetchDrafts(selectedUserId, filterOnlyProblem ? selectedProblemId : undefined)}
              loading={loading}
            >
              Обновить список
            </Button>
          </Group>
        </Paper>
      )}

      <Grid gutter="lg">
        {/* Left column: Editor */}
        <Grid.Col span={{base: 12, md: 7}}>
          <Paper withBorder p="md" radius="md" shadow="xs">
            <Stack gap="md">
              <Group justify="space-between" wrap="wrap">
                <Select
                  label="Задача"
                  placeholder="Выберите задачу"
                  data={problemOptions}
                  value={selectedProblemId}
                  onChange={(val) => {
                    setSelectedProblemId(val);
                    if (filterOnlyProblem) {
                      fetchDrafts(selectedUserId, val);
                    }
                  }}
                  style={{flex: 1, minWidth: 200}}
                />
                <Select
                  label="Язык"
                  data={languages}
                  value={language}
                  onChange={(val) => val && setLanguage(val)}
                  style={{width: 140}}
                />
              </Group>

              <Box className={classes.editorContainer}>
                {mounted ? (
                  <TypedEditor
                    value={code}
                    onValueChange={(val: string) => setCode(val)}
                    highlight={(codeText: string) => highlightCode(codeText, language)}
                    padding={12}
                    className={classes.codeEditor}
                    textareaClassName="nodrag"
                    style={{minHeight: "100%"}}
                    placeholder="// Введите или вставьте ваш код здесь..."
                  />
                ) : (
                  <Text c="dimmed" p="md">Загрузка редактора...</Text>
                )}
              </Box>

              <Group justify="space-between" wrap="wrap">
                <Group gap="xs">
                  <Button
                    leftSection={<IconDeviceFloppy size="1.1rem" />}
                    onClick={handleSaveDraft}
                    loading={isPending}
                    disabled={isSaveDisabled}
                  >
                    Сохранить черновик
                  </Button>
                  <Tooltip label="Копировать код из редактора">
                    <Button
                      variant="light"
                      leftSection={editorCopied ? <IconCheck size="1.1rem" /> : <IconCopy size="1.1rem" />}
                      onClick={() => handleCopyCode(code)}
                      disabled={!code.trim()}
                    >
                      {editorCopied ? "Скопировано" : "Копировать код"}
                    </Button>
                  </Tooltip>
                </Group>
                <Button
                  variant="subtle"
                  color="gray"
                  onClick={() => setCode("")}
                  disabled={!code}
                  size="xs"
                >
                  Очистить
                </Button>
              </Group>
            </Stack>
          </Paper>
        </Grid.Col>

        {/* Right column: Saved drafts */}
        <Grid.Col span={{base: 12, md: 5}}>
          <Paper withBorder p="md" radius="md" shadow="xs" style={{minHeight: 400}}>
            <Stack gap="sm">
              <Group justify="space-between" wrap="wrap">
                <Group gap="xs">
                  <IconFileCode size="1.2rem" />
                  <Text fw={600}>История черновиков</Text>
                  <Badge variant="light" size="sm">
                    {filteredDrafts.length}
                  </Badge>
                </Group>
                <Switch
                  label="Только для выбранной задачи"
                  size="xs"
                  checked={filterOnlyProblem}
                  onChange={(e) => {
                    const checked = e.currentTarget.checked;
                    setFilterOnlyProblem(checked);
                    fetchDrafts(selectedUserId, checked ? selectedProblemId : undefined);
                  }}
                />
              </Group>

              <Divider />

              {loading && (
                <Stack align="center" py="xl">
                  <Loader size="md" />
                  <Text size="sm" c="dimmed">Загрузка черновиков...</Text>
                </Stack>
              )}

              {!loading && filteredDrafts.length === 0 && (
                <Stack align="center" py="xl" gap="xs">
                  <IconInfoCircle size="2rem" color="gray" />
                  <Text size="sm" c="dimmed" ta="center">
                    {filterOnlyProblem
                      ? "Для выбранной задачи пока нет сохраненных черновиков."
                      : "У вас пока нет сохраненных черновиков в этом контесте."}
                  </Text>
                </Stack>
              )}

              {!loading && filteredDrafts.length > 0 && (
                <Stack gap="sm" style={{maxHeight: 520, overflowY: "auto"}}>
                  {filteredDrafts.map((draft) => {
                    const problemLetter = draft.position ? numberToLetters(draft.position) : "?";
                    const isCopied = copiedDraftId === draft.id;
                    const isDeleting = deletingId === draft.id;
                    const langLabel = languageIdToLabel[draft.language] || "Код";

                    return (
                      <Card
                        key={draft.id}
                        withBorder
                        padding="sm"
                        radius="md"
                        className={classes.draftCard}
                      >
                        <Stack gap="xs">
                          <Group justify="space-between" align="flex-start" wrap="nowrap">
                            <div>
                              <Group gap="xs">
                                <Badge size="sm" color="blue" variant="filled">
                                  {problemLetter}
                                </Badge>
                                <Text fw={600} size="sm" lineClamp={1}>
                                  {draft.problem_title || `Задача ${problemLetter}`}
                                </Text>
                              </Group>
                              <Group gap="xs" mt={4}>
                                <Badge size="xs" variant="outline">
                                  {langLabel}
                                </Badge>
                                <Text size="xs" c="dimmed">
                                  {new Date(draft.created_at).toLocaleString()}
                                </Text>
                                {isManager && draft.username && (
                                  <Badge size="xs" color="gray" variant="light">
                                    @{draft.username}
                                  </Badge>
                                )}
                              </Group>
                            </div>
                            <Group gap={6} wrap="nowrap">
                              <Tooltip label="Загрузить в редактор">
                                <Button
                                  variant="light"
                                  size="xs"
                                  onClick={() => handleLoadDraft(draft)}
                                >
                                  Открыть
                                </Button>
                              </Tooltip>
                              <Tooltip label={isCopied ? "Скопировано!" : "Копировать код"}>
                                <Button
                                  variant="subtle"
                                  size="xs"
                                  p={6}
                                  onClick={() => handleCopyCode(draft.code, draft.id)}
                                >
                                  {isCopied ? <IconCheck size="1rem" color="green" /> : <IconCopy size="1rem" />}
                                </Button>
                              </Tooltip>
                              {(!isContestEnded || isManager) && (
                                <Tooltip label="Удалить черновик">
                                  <Button
                                    variant="subtle"
                                    color="red"
                                    size="xs"
                                    p={6}
                                    onClick={() => handleDeleteDraft(draft.id)}
                                    loading={isDeleting}
                                  >
                                    <IconTrash size="1rem" />
                                  </Button>
                                </Tooltip>
                              )}
                            </Group>
                          </Group>

                          <Box className={classes.draftCodePreview}>
                            {draft.code.slice(0, 300)}
                            {draft.code.length > 300 ? "..." : ""}
                          </Box>
                        </Stack>
                      </Card>
                    );
                  })}
                </Stack>
              )}
            </Stack>
          </Paper>
        </Grid.Col>
      </Grid>
    </Stack>
  );
};
