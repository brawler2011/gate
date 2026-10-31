import {ActionIcon, Badge, Box, Center, Container, Group, Select, Skeleton, Stack, Table, Text, TextInput, Title, Tooltip} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconRefresh, IconSearch, IconX} from "@tabler/icons-react";
import {useQuery} from "@tanstack/react-query";
import {useState} from "react";
import {useLocation, useNavigate, useSearchParams, Link} from "react-router-dom";

import {RoutePagination} from '@/components/shared/Pagination';
import {SubmissionDetailsModal} from '@/components/submissions';
import {api} from "@/lib/api";
import {LangString, ShortVerdictString, StateColor, StateString, TimeBeautify} from "@/lib/lib";

import classes from "./AdminPage.module.css";

import type {SubmissionModel} from "@/contracts/core/v1/models";
import type {ReactNode} from "react";

type AdminSubmissionsContentProps = {
  page: number;
  state?: string;
  language?: string;
  contestId?: string;
  problemId?: string;
  userId?: string;
};

export const AdminSubmissionsContent = ({
  page,
  state,
  language,
  contestId,
  problemId,
  userId,
}: AdminSubmissionsContentProps): ReactNode => {
  const navigate = useNavigate();
  const {pathname} = useLocation();
  const [searchParams] = useSearchParams();
  const [rejudgingId, setRejudgingId] = useState<string | null>(null);
  const [selectedSubmissionId, setSelectedSubmissionId] = useState<string | null>(null);

  // Filter input states
  const [selectedState, setSelectedState] = useState<string>(state || "all");
  const [selectedLanguage, setSelectedLanguage] = useState<string>(language || "all");
  const [contestFilter, setContestFilter] = useState<string>(contestId || "");
  const [problemFilter, setProblemFilter] = useState<string>(problemId || "");
  const [userFilter, setUserFilter] = useState<string>(userId || "");

  const updateFilters = (updates: Record<string, string | undefined>) => {
    const params = new URLSearchParams(searchParams.toString());
    params.delete("page"); // Reset page on filter change

    Object.entries(updates).forEach(([key, val]) => {
      if (!val || val === "all") {
        params.delete(key);
      } else {
        params.set(key, val);
      }
    });

    navigate(`${pathname}?${params.toString()}`);
  };

  const handleClearFilters = () => {
    setSelectedState("all");
    setSelectedLanguage("all");
    setContestFilter("");
    setProblemFilter("");
    setUserFilter("");
    navigate(pathname);
  };

  // Fetch submissions
  const {data, error, isLoading, refetch: mutate} = useQuery({queryKey: ["admin-submissions", page, state, language, contestId, problemId, userId], queryFn: async () => {
    const [err, res] = await api.listSubmissions({
      page,
      pageSize: 10,
      contestId: contestId || undefined,
      problemId: problemId || undefined,
      state: state as NonNullable<Parameters<typeof api.listSubmissions>[0]>["state"],
      language: language as NonNullable<Parameters<typeof api.listSubmissions>[0]>["language"]
    });

    if (err) {
      throw new Error(err.message);
    }
    return res;
  }});

  const submissions = (data?.submissions || []) as unknown as SubmissionModel[];
  const pagination = data?.pagination || {total: 0, page: page};

  const handleRejudge = async (e: React.MouseEvent, submission: SubmissionModel) => {
    e.stopPropagation();
    if (!submission.organization_login || !submission.contest_login) {
      return;
    }
    setRejudgingId(submission.id);
    try {
      const [err] = await api.rejudgeSubmission(submission.organization_login, submission.contest_login, submission.id);

      if (err) {
        notifications.show({
          title: "Ошибка",
          message: err.message || "Не удалось перепроверить посылку",
          color: "red",
        });
        return;
      }

      notifications.show({
        title: "Успех",
        message: "Посылка отправлена на перепроверку",
        color: "green",
      });

      mutate();
    } finally {
      setRejudgingId(null);
    }
  };

  if (error) {
    return (
      <Container size="xl" py="md">
        <Center>
          <Stack align="center">
            <Title order={2}>Ошибка при загрузке посылок</Title>
            <Text c="dimmed">{error.message}</Text>
          </Stack>
        </Center>
      </Container>
    );
  }

  const totalPages = pagination.total || 1;
  const hasActiveFilters = !!(state || language || contestId || problemId || userId);

  const queryParams: Record<string, string | number | undefined> = {};
  if (state) {
    queryParams.state = state;
  }
  if (language) {
    queryParams.language = language;
  }
  if (contestId) {
    queryParams.contestId = contestId;
  }
  if (problemId) {
    queryParams.problemId = problemId;
  }
  if (userId) {
    queryParams.userId = userId;
  }

  return (
    <Container size="xl" py="md">
      <Stack gap="md">
        {/* Filters Bar */}
        <Group wrap="wrap" align="flex-end" gap="sm">
          <Select
            label="Вердикт"
            size="xs"
            value={selectedState}
            onChange={(val) => {
              const v = val || "all";
              setSelectedState(v);
              updateFilters({state: v});
            }}
            data={[
              {value: "all", label: "Все вердикты"},
              {value: "OK", label: "OK (Успешно)"},
              {value: "WA", label: "WA (Неверный ответ)"},
              {value: "TLE", label: "TLE (Превышение времени)"},
              {value: "MLE", label: "MLE (Превышение памяти)"},
              {value: "CE", label: "CE (Ошибка компиляции)"},
              {value: "RE", label: "RE (Ошибка выполнения)"},
              {value: "SE", label: "SE (Ошибка системы)"},
            ]}
            style={{width: 170}}
          />

          <Select
            label="Язык"
            size="xs"
            value={selectedLanguage}
            onChange={(val) => {
              const v = val || "all";
              setSelectedLanguage(v);
              updateFilters({language: v});
            }}
            data={[
              {value: "all", label: "Все языки"},
              {value: "cpp", label: "C++"},
              {value: "python", label: "Python"},
              {value: "go", label: "Go"},
              {value: "java", label: "Java"},
            ]}
            style={{width: 140}}
          />

          <TextInput
            label="ID контеста"
            size="xs"
            placeholder="Фильтр по контесту..."
            value={contestFilter}
            onChange={(e) => setContestFilter(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                updateFilters({contestId: contestFilter});
              }
            }}
            style={{width: 160}}
          />

          <TextInput
            label="ID задачи"
            size="xs"
            placeholder="Фильтр по задаче..."
            value={problemFilter}
            onChange={(e) => setProblemFilter(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                updateFilters({problemId: problemFilter});
              }
            }}
            style={{width: 160}}
          />

          <TextInput
            label="ID пользователя"
            size="xs"
            placeholder="Фильтр по автору..."
            value={userFilter}
            onChange={(e) => setUserFilter(e.currentTarget.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                updateFilters({userId: userFilter});
              }
            }}
            style={{width: 160}}
          />

          <ActionIcon
            variant="light"
            color="blue"
            size="sm"
            title="Применить текстовые фильтры"
            onClick={() =>
              updateFilters({
                contestId: contestFilter,
                problemId: problemFilter,
                userId: userFilter,
              })
            }
          >
            <IconSearch size={14} />
          </ActionIcon>

          {hasActiveFilters && (
            <ActionIcon
              variant="subtle"
              color="red"
              size="sm"
              title="Сбросить все фильтры"
              onClick={handleClearFilters}
            >
              <IconX size={14} />
            </ActionIcon>
          )}
        </Group>

        {isLoading && (
          <Stack gap="sm">
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
          </Stack>
        )}
        {!isLoading && submissions.length === 0 && (
          <Center py="xl">
            <Text c="dimmed">Посылки не найдены</Text>
          </Center>
        )}
        {!isLoading && submissions.length > 0 && (
          <>
            <Box className={classes.tableContainer}>
              <Table className={classes.table} verticalSpacing="xs">
                <Table.Thead className={classes.thead}>
                  <Table.Tr>
                    <Table.Th style={{width: "12%"}}>ID</Table.Th>
                    <Table.Th style={{width: "20%"}}>Задача</Table.Th>
                    <Table.Th style={{width: "15%"}}>Контест</Table.Th>
                    <Table.Th style={{width: "15%"}}>Отправитель</Table.Th>
                    <Table.Th style={{width: "10%"}}>Язык</Table.Th>
                    <Table.Th style={{width: "13%"}}>Вердикт</Table.Th>
                    <Table.Th style={{width: "10%"}}>Когда</Table.Th>
                    <Table.Th style={{width: "5%"}}>Действия</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody className={classes.tbody}>
                  {submissions.map((submission: SubmissionModel) => (
                    <Table.Tr key={submission.id}>
                      <Table.Td>
                        <Text
                          c="blue"
                          fw={500}
                          style={{cursor: "pointer"}}
                          onClick={() => setSelectedSubmissionId(submission.id)}
                        >
                          {submission.id.slice(0, 8)}...
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <Link
                          to={
                            submission.organization_login
                              ? `/${submission.organization_login}/problems/${submission.problem_id}`
                              : `/problems/${submission.problem_id}`
                          }
                          style={{textDecoration: "none"}}
                        >
                          <Text c="blue" lineClamp={1}>
                            {submission.problem_title || submission.problem_id.slice(0, 8)}
                          </Text>
                        </Link>
                      </Table.Td>
                      <Table.Td>
                        {submission.contest_login ? (
                          <Link
                            to={
                              submission.organization_login
                                ? `/${submission.organization_login}/${submission.contest_login}`
                                : `/${submission.contest_login}`
                            }
                            style={{textDecoration: "none"}}
                          >
                            <Text c="blue" lineClamp={1}>
                              {submission.contest_title || submission.contest_login}
                            </Text>
                          </Link>
                        ) : (
                          <Text c="dimmed">—</Text>
                        )}
                      </Table.Td>
                      <Table.Td>
                        <Link to={`/@${submission.username}`} style={{textDecoration: "none"}}>
                          <Text c="blue" lineClamp={1}>
                            {submission.username}
                          </Text>
                        </Link>
                      </Table.Td>
                      <Table.Td>
                        <Badge variant="light" color="gray" tt="none">
                          {LangString(submission.language)}
                        </Badge>
                      </Table.Td>
                      <Table.Td>
                        <Tooltip label={StateString(submission.state, submission.failed_test)} withArrow>
                          <Text
                            c={StateColor(submission.state)}
                            fw={600}
                            style={{cursor: "pointer"}}
                            onClick={() => setSelectedSubmissionId(submission.id)}
                          >
                            {submission.frozen ? "Заморожено" : ShortVerdictString(submission.state, submission.failed_test)}
                          </Text>
                        </Tooltip>
                      </Table.Td>
                      <Table.Td>
                        <Text className={classes.dateCell}>
                          {TimeBeautify(submission.created_at)}
                        </Text>
                      </Table.Td>
                      <Table.Td onClick={(e) => e.stopPropagation()}>
                        <Tooltip label="Перепроверить посылку">
                          <ActionIcon
                            variant="subtle"
                            color="orange"
                            loading={rejudgingId === submission.id}
                            onClick={(e) => handleRejudge(e, submission)}
                          >
                            <IconRefresh size={16} />
                          </ActionIcon>
                        </Tooltip>
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </Box>

            {totalPages > 1 && (
              <Stack align="center" gap="md">
                <RoutePagination
                  pagination={{
                    page: page,
                    total: totalPages,
                  }}
                  baseUrl="/admin/submissions"
                  queryParams={queryParams}
                />
              </Stack>
            )}
          </>
        )}
      </Stack>

      <SubmissionDetailsModal
        submissionId={selectedSubmissionId}
        opened={Boolean(selectedSubmissionId)}
        onClose={() => setSelectedSubmissionId(null)}
      />
    </Container>
  );
};
