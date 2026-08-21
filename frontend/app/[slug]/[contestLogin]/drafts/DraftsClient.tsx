"use client";

import {
  ActionIcon,
  Badge,
  Box,
  Button,
  Card,
  Divider,
  Grid,
  Group,
  Loader,
  Paper,
  Stack,
  Text,
  Textarea,
  Tooltip,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {
  IconCheck,
  IconCopy,
  IconDeviceFloppy,
  IconFileCode,
  IconInfoCircle,
  IconPencil,
  IconTrash,
} from "@tabler/icons-react";
import React, {useCallback, useState, useTransition, type ReactNode} from "react";

import {api} from "@/lib/api";

import classes from "./DraftsClient.module.css";

import type {ContestDraftModel, ContestModel} from "@/contracts/core/v1";

type Props = {
  contest: ContestModel;
  isManager: boolean;
  isContestEnded: boolean;
  initialDrafts: ContestDraftModel[];
};

export const DraftsClient = ({
  contest,
  isManager,
  isContestEnded,
  initialDrafts,
}: Props): ReactNode => {
  const [code, setCode] = useState<string>("");
  const [drafts, setDrafts] = useState<ContestDraftModel[]>(initialDrafts);
  const [loading, setLoading] = useState<boolean>(false);
  const [copiedDraftId, setCopiedDraftId] = useState<string | null>(null);
  const [editorCopied, setEditorCopied] = useState<boolean>(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const [isPending, startTransition] = useTransition();

  const fetchDrafts = useCallback(async () => {
    setLoading(true);
    const [error, data] = await api.listContestDrafts({
      orgLogin: contest.organization_login,
      contestLogin: contest.login,
      page: 1,
      pageSize: 50,
    });
    setLoading(false);
    if (!error && data?.drafts) {
      setDrafts(data.drafts);
    }
  }, [contest.organization_login, contest.login]);

  const handleSaveDraft = async () => {
    if (!code.trim()) {
      notifications.show({
        title: "Ошибка",
        message: "Код черновика не может быть пустым",
        color: "red",
      });
      return;
    }
    if (code.length > 64 * 1024) {
      notifications.show({
        title: "Ошибка",
        message: "Превышен максимальный размер кода (64 КБ)",
        color: "red",
      });
      return;
    }

    startTransition(async () => {
      const [error, response] = await api.createContestDraft({
        orgLogin: contest.organization_login,
        contestLogin: contest.login,
        requestBody: {
          code,
        },
      });

      if (error) {
        notifications.show({
          title: "Ошибка",
          message: error.message || "Ошибка при сохранении черновика",
          color: "red",
        });
      } else if (response?.id) {
        notifications.show({
          title: "Успешно",
          message: "Черновик успешно сохранен!",
          color: "green",
        });
        await fetchDrafts();
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
      notifications.show({
        title: "Успешно",
        message: "Черновик удален",
        color: "green",
      });
    } else {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось удалить черновик",
        color: "red",
      });
    }
  };

  const handleLoadDraft = (draft: ContestDraftModel) => {
    setCode(draft.code);
    notifications.show({
      title: "Загружено",
      message: "Черновик загружен в редактор",
      color: "blue",
    });
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
      notifications.show({
        title: "Скопировано",
        message: "Код скопирован в буфер обмена",
        color: "green",
      });
    } catch {
      notifications.show({
        title: "Ошибка",
        message: "Не удалось скопировать код в буфер обмена",
        color: "red",
      });
    }
  };

  const isSaveDisabled =
    (isContestEnded && !isManager) ||
    !code.trim() ||
    isPending;

  return (
    <Stack>
      {isContestEnded && (
        <Box>
          <Badge color="orange" size="lg" variant="light">
            Контест завершен (только чтение)
          </Badge>
        </Box>
      )}

      <Grid gutter="lg">
        {/* Left column: Editor */}
        <Grid.Col span={{base: 12, md: 7}}>
          <Paper withBorder p="md" radius="md" shadow="xs">
            <Stack gap="md">
              <Group justify="flex-start" wrap="wrap" gap="xs">
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

              <Textarea
                value={code}
                onChange={(e) => setCode(e.currentTarget.value)}
                placeholder="// Введите или вставьте ваш код здесь..."
                minRows={20}
                autosize
                className={classes.codeTextarea}
              />
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
                    {drafts.length}
                  </Badge>
                </Group>
              </Group>

              <Divider />

              {loading && (
                <Stack align="center" py="xl">
                  <Loader size="md" />
                  <Text size="sm" c="dimmed">Загрузка черновиков...</Text>
                </Stack>
              )}

              {!loading && drafts.length === 0 && (
                <Stack align="center" py="xl" gap="xs">
                  <IconInfoCircle size="2rem" color="gray" />
                  <Text size="sm" c="dimmed" ta="center">
                    У вас пока нет сохраненных черновиков в этом контесте.
                  </Text>
                </Stack>
              )}

              {!loading && drafts.length > 0 && (
                <Stack gap="sm" style={{maxHeight: 520, overflowY: "auto"}}>
                  {drafts.map((draft) => {
                    const isCopied = copiedDraftId === draft.id;
                    const isDeleting = deletingId === draft.id;

                    return (
                      <Card
                        key={draft.id}
                        withBorder
                        padding="xs"
                        radius="md"
                        className={classes.draftCard}
                      >
                        <Group justify="space-between" align="center" wrap="nowrap">
                          <Badge size="sm" variant="light" color="blue">
                            {new Date(draft.created_at).toLocaleString()}
                          </Badge>
                          <Group gap={6} wrap="nowrap">
                            <Tooltip label="Загрузить в редактор">
                              <ActionIcon
                                variant="light"
                                size="sm"
                                onClick={() => handleLoadDraft(draft)}
                                aria-label="Загрузить в редактор"
                              >
                                <IconPencil size="1rem" />
                              </ActionIcon>
                            </Tooltip>
                            <Tooltip label={isCopied ? "Скопировано!" : "Копировать код"}>
                              <ActionIcon
                                variant="subtle"
                                size="sm"
                                onClick={() => handleCopyCode(draft.code, draft.id)}
                                aria-label="Копировать код"
                              >
                                {isCopied ? <IconCheck size="1rem" color="green" /> : <IconCopy size="1rem" />}
                              </ActionIcon>
                            </Tooltip>
                            {(!isContestEnded || isManager) && (
                              <Tooltip label="Удалить черновик">
                                <ActionIcon
                                  variant="subtle"
                                  color="red"
                                  size="sm"
                                  onClick={() => handleDeleteDraft(draft.id)}
                                  loading={isDeleting}
                                  aria-label="Удалить черновик"
                                >
                                  <IconTrash size="1rem" />
                                </ActionIcon>
                              </Tooltip>
                            )}
                          </Group>
                        </Group>
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
