"use client";

import {
  ActionIcon,
  Badge,
  Button,
  Card,
  Center,
  Group,
  Paper,
  Select,
  Stack,
  Tabs,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {
  IconCheck,
  IconClock,
  IconHelpCircle,
  IconMessagePlus,
  IconPlus,
  IconSpeakerphone,
  IconTrash,
} from "@tabler/icons-react";
import {useCallback, useEffect, useState, type ReactNode} from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import {useContestEventsWebSocket} from "@/hooks/useContestEventsWebSocket";
import {api} from "@/lib/api";

import {AnswerClarificationModal} from "./AnswerClarificationModal";
import {AskClarificationModal} from "./AskClarificationModal";
import classes from "./ContestMessagesClient.module.css";
import {CreateAnnouncementModal} from "./CreateAnnouncementModal";

import type {
  ContestAnnouncementModel,
  ContestClarificationModel,
  ContestModel,
  ContestProblemListItemModel,
  UserModel,
} from "@/contracts/core/v1";

export interface ContestMessagesClientProps {
  contest: ContestModel;
  orgLogin: string;
  contestLogin: string;
  user: UserModel | null;
  isModerator: boolean;
  problems?: ContestProblemListItemModel[];
  initialAnnouncements?: ContestAnnouncementModel[];
  initialClarifications?: ContestClarificationModel[];
}

export const ContestMessagesClient = ({
  contest,
  orgLogin,
  contestLogin,
  user,
  isModerator,
  problems = [],
  initialAnnouncements = [],
  initialClarifications = [],
}: ContestMessagesClientProps): ReactNode => {
  const [activeTab, setActiveTab] = useState<string | null>("announcements");

  // Announcements state
  const [announcements, setAnnouncements] = useState<ContestAnnouncementModel[]>(initialAnnouncements);
  const [announcementsLoading, setAnnouncementsLoading] = useState(false);

  // Clarifications state
  const [clarifications, setClarifications] = useState<ContestClarificationModel[]>(initialClarifications);
  const [clarificationsLoading, setClarificationsLoading] = useState(false);

  // Modals
  const [createAnnOpened, setCreateAnnOpened] = useState(false);
  const [askClarOpened, setAskClarOpened] = useState(false);
  const [answeringClar, setAnsweringClar] = useState<ContestClarificationModel | null>(null);

  // Filters for clarifications
  const [statusFilter, setStatusFilter] = useState<string>("");
  const [problemFilter, setProblemFilter] = useState<string>("");

  const refreshAnnouncements = useCallback(async () => {
    setAnnouncementsLoading(true);
    const [err, res] = await api.listContestAnnouncements({
      orgLogin,
      contestLogin,
      page: 1,
      pageSize: 100,
    });
    setAnnouncementsLoading(false);
    if (!err && res) {
      setAnnouncements(res.announcements);
    }
  }, [orgLogin, contestLogin]);

  const refreshClarifications = useCallback(async () => {
    if (!user?.id) {
      return;
    }
    setClarificationsLoading(true);
    const [err, res] = await api.listContestClarifications({
      orgLogin,
      contestLogin,
      page: 1,
      pageSize: 100,
      status: statusFilter || undefined,
      problemId: problemFilter || undefined,
    });
    setClarificationsLoading(false);
    if (!err && res) {
      setClarifications(res.clarifications);
    }
  }, [orgLogin, contestLogin, user?.id, statusFilter, problemFilter]);

  // Refetch clarifications when filters change
  useEffect(() => {
    refreshClarifications();
  }, [refreshClarifications]);

  // Real-time WebSocket listener
  useContestEventsWebSocket({
    contestId: contest.id,
    enabled: !!user?.id,
    currentUserId: user?.id,
    isModerator,
    onEvent: () => {
      refreshAnnouncements();
      refreshClarifications();
    },
  });

  const pendingClarificationsCount = clarifications.filter(
    (c: ContestClarificationModel) => c.status === "pending"
  ).length;

  const allowClarifications = contest.allow_clarifications ?? true;

  const handleDeleteAnnouncement = async (id: string) => {
    if (!confirm("Вы действительно хотите удалить это объявление?")) {
      return;
    }

    const [err] = await api.deleteContestAnnouncement({
      orgLogin,
      contestLogin,
      announcementId: id,
    });

    if (err) {
      notifications.show({
        title: "Ошибка",
        message: err.message || "Не удалось удалить объявление",
        color: "red",
      });
      return;
    }

    notifications.show({
      title: "Удалено",
      message: "Объявление удалено",
      color: "green",
    });

    refreshAnnouncements();
  };

  const formatDate = (iso: string) => {
    try {
      const d = new Date(iso);
      return d.toLocaleString("ru-RU", {
        day: "2-digit",
        month: "2-digit",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return iso;
    }
  };

  const renderClarificationsTabBadge = () => {
    if (isModerator && pendingClarificationsCount > 0) {
      return (
        <Badge size="xs" variant="filled" circle color="yellow">
          {pendingClarificationsCount}
        </Badge>
      );
    }
    if (clarifications.length > 0) {
      return (
        <Badge size="xs" variant="light" circle>
          {clarifications.length}
        </Badge>
      );
    }
    return null;
  };

  const renderAnnouncementsContent = () => {
    if (announcementsLoading && announcements.length === 0) {
      return (
        <Text c="dimmed" size="sm">
          Загрузка объявлений...
        </Text>
      );
    }
    if (announcements.length === 0) {
      return (
        <Paper withBorder p="xl" radius="md">
          <Center>
            <Stack align="center" gap="xs">
              <IconSpeakerphone size={36} color="var(--mantine-color-dimmed)" />
              <Text c="dimmed" size="sm">
                Объявлений от жюри пока нет
              </Text>
            </Stack>
          </Center>
        </Paper>
      );
    }
    return (
      <Stack gap="md">
        {announcements.map((ann: ContestAnnouncementModel) => (
          <Card key={ann.id} withBorder radius="md" p="md" className={classes.card}>
            <div className={classes.cardHeader}>
              <Stack gap={4}>
                <Group gap="xs" align="center">
                  <Title order={5}>{ann.title}</Title>
                  {ann.problem_letter && (
                    <Badge size="sm" variant="light" color="blue">
                      Задача {ann.problem_letter}
                      {ann.problem_title ? ` — ${ann.problem_title}` : ""}
                    </Badge>
                  )}
                </Group>
                <div className={classes.cardMeta}>
                  <Text size="xs" c="dimmed">
                    {ann.author_username || "Жюри"}
                  </Text>
                  <Text size="xs" c="dimmed">
                    •
                  </Text>
                  <Text size="xs" c="dimmed">
                    {formatDate(ann.created_at)}
                  </Text>
                </div>
              </Stack>

              {isModerator && (
                <Tooltip label="Удалить объявление">
                  <ActionIcon
                    variant="subtle"
                    color="red"
                    size="sm"
                    onClick={() => handleDeleteAnnouncement(ann.id)}
                  >
                    <IconTrash size={16} />
                  </ActionIcon>
                </Tooltip>
              )}
            </div>

            <Paper withBorder mt="sm" p="sm" bg="var(--mantine-color-default-hover)" radius="sm">
              <div className={classes.markdownBody}>
                <ReactMarkdown remarkPlugins={[remarkGfm]}>
                  {ann.body}
                </ReactMarkdown>
              </div>
            </Paper>
          </Card>
        ))}
      </Stack>
    );
  };

  const renderClarificationsContent = () => {
    if (clarificationsLoading && clarifications.length === 0) {
      return (
        <Text c="dimmed" size="sm">
          Загрузка вопросов...
        </Text>
      );
    }
    if (clarifications.length === 0) {
      const emptyText = isModerator
        ? "Нет вопросов от участников"
        : "Вы еще не задавали вопросов жюри";

      return (
        <Paper withBorder p="xl" radius="md">
          <Center>
            <Stack align="center" gap="xs">
              <IconHelpCircle size={36} color="var(--mantine-color-dimmed)" />
              <Text c="dimmed" size="sm">
                {emptyText}
              </Text>
              {!isModerator && allowClarifications && (
                <Button
                  size="xs"
                  variant="light"
                  onClick={() => setAskClarOpened(true)}
                  mt="xs"
                >
                  Задать первый вопрос
                </Button>
              )}
            </Stack>
          </Center>
        </Paper>
      );
    }
    return (
      <Stack gap="md">
        {clarifications.map((clar: ContestClarificationModel) => {
          const isPending = clar.status === "pending";

          return (
            <Card
              key={clar.id}
              withBorder
              radius="md"
              p="md"
              className={classes.card}
            >
              <div className={classes.cardHeader}>
                <Group gap="xs" align="center">
                  {isPending ? (
                    <Badge
                      color="yellow"
                      variant="light"
                      leftSection={<IconClock size={12} />}
                    >
                      Ожидает ответа
                    </Badge>
                  ) : (
                    <Badge
                      color="teal"
                      variant="light"
                      leftSection={<IconCheck size={12} />}
                    >
                      Отвечен
                    </Badge>
                  )}

                  {clar.problem_letter && (
                    <Badge size="sm" variant="outline">
                      Задача {clar.problem_letter}
                      {clar.problem_title ? ` (${clar.problem_title})` : ""}
                    </Badge>
                  )}

                  {isModerator && clar.username && (
                    <Text size="xs" fw={500} c="dimmed">
                      от {clar.username}
                    </Text>
                  )}
                  <Text size="xs" c="dimmed">
                    • {formatDate(clar.created_at)}
                  </Text>
                </Group>

                {isModerator && (
                  <Button
                    size="xs"
                    variant={isPending ? "filled" : "light"}
                    color="blue"
                    onClick={() => setAnsweringClar(clar)}
                  >
                    {isPending ? "Ответить" : "Изменить ответ"}
                  </Button>
                )}
              </div>

              <div className={classes.questionBox} style={{marginTop: 10}}>
                <Text size="xs" fw={700} c="dimmed" mb={2}>
                  ВОПРОС:
                </Text>
                <Text size="sm" style={{whiteSpace: "pre-wrap"}}>
                  {clar.question}
                </Text>
              </div>

              {clar.answer && (
                <div className={classes.answerBox} style={{marginTop: 8}}>
                  <Group justify="space-between" mb={2}>
                    <Text size="xs" fw={700} c="teal">
                      ОТВЕТ ЖЮРИ:
                    </Text>
                    {clar.answered_at && (
                      <Text size="xs" c="dimmed">
                        {formatDate(clar.answered_at)}
                        {clar.answered_by_username
                          ? ` (${clar.answered_by_username})`
                          : ""}
                      </Text>
                    )}
                  </Group>
                  <Text size="sm" style={{whiteSpace: "pre-wrap"}}>
                    {clar.answer}
                  </Text>
                </div>
              )}
            </Card>
          );
        })}
      </Stack>
    );
  };

  return (
    <div className={classes.container}>
      <div className={classes.header}>
        <Title order={3}>Сообщения контеста</Title>
        <Group gap="xs">
          {user?.id && allowClarifications && (
            <Button
              leftSection={<IconMessagePlus size={16} />}
              variant="light"
              color="blue"
              onClick={() => setAskClarOpened(true)}
            >
              Задать вопрос
            </Button>
          )}
          {isModerator && (
            <Button
              leftSection={<IconPlus size={16} />}
              color="blue"
              onClick={() => setCreateAnnOpened(true)}
            >
              Создать объявление
            </Button>
          )}
        </Group>
      </div>

      <Tabs value={activeTab} onChange={setActiveTab}>
        <Tabs.List mb="md">
          <Tabs.Tab
            value="announcements"
            leftSection={<IconSpeakerphone size={16} />}
            rightSection={
              announcements.length > 0 ? (
                <Badge size="xs" variant="filled" circle color="blue">
                  {announcements.length}
                </Badge>
              ) : null
            }
          >
            Объявления
          </Tabs.Tab>

          {user?.id && (
            <Tabs.Tab
              value="clarifications"
              leftSection={<IconHelpCircle size={16} />}
              rightSection={renderClarificationsTabBadge()}
            >
              {isModerator ? "Вопросы участников" : "Вопросы жюри"}
            </Tabs.Tab>
          )}
        </Tabs.List>

        {/* Announcements Tab */}
        <Tabs.Panel value="announcements">
          {renderAnnouncementsContent()}
        </Tabs.Panel>

        {/* Clarifications Tab */}
        {user?.id && (
          <Tabs.Panel value="clarifications">
            {isModerator && (
              <Group mb="md" gap="sm">
                <Select
                  placeholder="Статус"
                  size="xs"
                  w={180}
                  value={statusFilter}
                  onChange={(val) => setStatusFilter(val || "")}
                  data={[
                    {value: "", label: "Все статусы"},
                    {value: "pending", label: "Ожидают ответа"},
                    {value: "answered", label: "Отвеченные"},
                  ]}
                  clearable
                />
                {problems.length > 0 && (
                  <Select
                    placeholder="Задача"
                    size="xs"
                    w={220}
                    value={problemFilter}
                    onChange={(val) => setProblemFilter(val || "")}
                    data={[
                      {value: "", label: "Все задачи"},
                      ...problems.map((p) => {
                        const letter = String.fromCharCode(65 + (p.position ?? 0));
                        return {
                          value: p.problem_id,
                          label: `${letter}. ${p.title || "Задача"}`,
                        };
                      }),
                    ]}
                    clearable
                  />
                )}
              </Group>
            )}

            {renderClarificationsContent()}
          </Tabs.Panel>
        )}
      </Tabs>

      {/* Modals */}
      <CreateAnnouncementModal
        opened={createAnnOpened}
        onClose={() => setCreateAnnOpened(false)}
        orgLogin={orgLogin}
        contestLogin={contestLogin}
        problems={problems}
        onSuccess={() => {
          refreshAnnouncements();
        }}
      />

      <AskClarificationModal
        opened={askClarOpened}
        onClose={() => setAskClarOpened(false)}
        orgLogin={orgLogin}
        contestLogin={contestLogin}
        problems={problems}
        onSuccess={() => {
          refreshClarifications();
          setActiveTab("clarifications");
        }}
      />

      <AnswerClarificationModal
        opened={!!answeringClar}
        onClose={() => setAnsweringClar(null)}
        orgLogin={orgLogin}
        contestLogin={contestLogin}
        clarification={answeringClar}
        onSuccess={() => {
          refreshClarifications();
          refreshAnnouncements();
        }}
      />
    </div>
  );
};
