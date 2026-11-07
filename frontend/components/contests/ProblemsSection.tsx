import {ActionIcon, Autocomplete, Badge, Button, Center, Divider, Group, Loader, Modal, Select, Stack, Table, Text, Tooltip} from "@mantine/core";
import {useDebouncedValue} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {IconDeviceFloppy, IconEdit, IconGripVertical, IconPlus, IconRefresh, IconRotate2, IconTrash} from "@tabler/icons-react";
import {useInfiniteQuery, useQueryClient} from "@tanstack/react-query";
import {useEffect, useState} from "react";
import {useRevalidator} from "react-router-dom";

import {DownloadStatementsButton} from "@/components/contests/DownloadStatementsButton";
import {StatusMessage} from "@/components/shared/StatusMessage";
import {api, unwrap} from "@/lib/api";
import {numberToLetters} from "@/lib/lib";

import classes from "./ProblemsSection.module.css";

import type * as corev1 from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

interface ProblemsSectionProps {
  orgLogin: string;
  contestLogin: string;
  contestId?: string;
  initialProblems: Array<corev1.ContestProblemListItemModel>;
}

const checkHasChanges = (
  current: corev1.ContestProblemListItemModel[],
  saved: corev1.ContestProblemListItemModel[],
): boolean => {
  if (current.length !== saved.length) {
    return true;
  }
  for (let i = 0; i < current.length; i++) {
    if (
      current[i].problem_id !== saved[i].problem_id ||
      current[i].position !== saved[i].position
    ) {
      return true;
    }
  }
  return false;
};

export const ProblemsSection = ({
  orgLogin,
  contestLogin,
  initialProblems,
}: ProblemsSectionProps): ReactNode => {
  const revalidator = useRevalidator();
  const queryClient = useQueryClient();
  const contestQueryKey = api.queryOptions("get", "/organizations/{org_login}/contests/{contest_login}", {
    params: {path: {org_login: orgLogin, contest_login: contestLogin}},
  }).queryKey;
  const [problems, setProblems] = useState(initialProblems);
  const [savedProblems, setSavedProblems] = useState(initialProblems);
  const [hasUnsavedChanges, setHasUnsavedChanges] = useState(false);
  const [savingOrder, setSavingOrder] = useState(false);

  const [draggedIndex, setDraggedIndex] = useState<number | null>(null);
  const [dragOverIndex, setDragOverIndex] = useState<number | null>(null);

  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedQuery] = useDebouncedValue(searchQuery, 300);
  const [searchResults, setSearchResults] = useState<corev1.ProblemsListItemModel[]>([]);
  const [searching, setSearching] = useState(false);
  const [selectedProblemId, setSelectedProblemId] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  // Modal for replacing package version
  const [replaceTarget, setReplaceTarget] = useState<{ problemId: string; title: string } | null>(null);
  const [selectedPackageId, setSelectedPackageId] = useState<string | null>(null);
  const [replacing, setReplacing] = useState(false);

  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  useEffect(() => {
    setProblems(initialProblems);
    setSavedProblems(initialProblems);
    setHasUnsavedChanges(false);
  }, [initialProblems]);

  useEffect(() => {
    const searchProblemsAsync = async () => {
      if (!debouncedQuery || debouncedQuery.length < 2) {
        setSearchResults([]);
        return;
      }

      setSearching(true);
      const {data: response, error} = await api.GET("/problems", {
        params: {
          query: {
            page: 1,
            pageSize: 10,
            search: debouncedQuery,
            owner: true,
          },
        },
      });
      setSearching(false);

      if (error) {
        console.error("Failed to search problems:", error);
        return;
      }

      setSearchResults(response?.problems || []);
    };

    searchProblemsAsync();
  }, [debouncedQuery]);

  const handleAddProblem = async () => {
    if (!selectedProblemId) {
      return;
    }

    setAdding(true);
    const {error} = await api.POST(
      "/organizations/{org_login}/contests/{contest_login}/problems",
      {
        params: {
          path: {org_login: orgLogin, contest_login: contestLogin},
          query: {problem_id: selectedProblemId},
        },
      },
    );
    setAdding(false);

    if (error) {
      console.error("Failed to add problem:", error);
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось добавить задачу",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось добавить задачу",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Задача добавлена в контест",
    });

    setSearchQuery("");
    setSelectedProblemId(null);
    void queryClient.invalidateQueries({queryKey: contestQueryKey});
    void revalidator.revalidate();
  };

  const handleDeleteProblem = async (problemId: string) => {
    setDeletingId(problemId);
    const {error} = await api.DELETE(
      "/organizations/{org_login}/contests/{contest_login}/problems/{problem_id}",
      {
        params: {path: {org_login: orgLogin, contest_login: contestLogin, problem_id: problemId}},
      },
    );
    setDeletingId(null);

    if (error) {
      console.error("Failed to delete problem:", error);
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось удалить задачу",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось удалить задачу",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Задача удалена из контеста",
    });

    void queryClient.invalidateQueries({queryKey: contestQueryKey});
    void revalidator.revalidate();
  };

  const handleDragStart = (e: React.DragEvent, index: number) => {
    e.dataTransfer.setData("text/plain", `${index}`);
    e.dataTransfer.effectAllowed = "move";
    setDraggedIndex(index);
  };

  const handleDragOver = (e: React.DragEvent, index: number) => {
    e.preventDefault();
    e.dataTransfer.dropEffect = "move";
    if (dragOverIndex !== index) {
      setDragOverIndex(index);
    }
  };

  const handleDragEnd = () => {
    setDraggedIndex(null);
    setDragOverIndex(null);
  };

  const handleDrop = (e: React.DragEvent, targetIndex: number) => {
    e.preventDefault();
    if (draggedIndex === null || draggedIndex === targetIndex) {
      setDraggedIndex(null);
      setDragOverIndex(null);
      return;
    }

    const updated = [...problems];
    const [moved] = updated.splice(draggedIndex, 1);
    updated.splice(targetIndex, 0, moved);

    const reindexed = updated.map((item, idx) => ({
      ...item,
      position: idx + 1,
    }));

    setProblems(reindexed);
    setHasUnsavedChanges(checkHasChanges(reindexed, savedProblems));
    setDraggedIndex(null);
    setDragOverIndex(null);
  };

  const handleRecalculatePositions = () => {
    const reindexed = problems.map((item, idx) => ({
      ...item,
      position: idx + 1,
    }));
    setProblems(reindexed);
    const changed = checkHasChanges(reindexed, savedProblems);
    setHasUnsavedChanges(changed);

    if (changed) {
      notifications.show({
        title: "Позиции пересчитаны",
        message: `Позиции перенумерованы (1..${reindexed.length}). Не забудьте сохранить порядок.`,
        color: "blue",
      });
    } else {
      notifications.show({
        title: "Позиции в порядке",
        message: "Все позиции уже идут последовательно без пропусков.",
        color: "gray",
      });
    }
  };

  const handleSaveOrder = async () => {
    setSavingOrder(true);
    const {error} = await api.PUT(
      "/organizations/{org_login}/contests/{contest_login}/problems/reorder",
      {
        params: {path: {org_login: orgLogin, contest_login: contestLogin}},
        body: {
          problems: problems.map((p) => ({
            problem_id: p.problem_id,
            position: p.position,
          })),
        },
      },
    );
    setSavingOrder(false);

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось сохранить порядок задач",
        color: "red",
      });
      return;
    }

    setSavedProblems(problems);
    setHasUnsavedChanges(false);
    notifications.show({
      title: "Успешно",
      message: "Порядок задач сохранён",
      color: "green",
    });
    void queryClient.invalidateQueries({queryKey: contestQueryKey});
    void revalidator.revalidate();
  };

  const handleResetOrder = () => {
    setProblems(savedProblems);
    setHasUnsavedChanges(false);
  };

  const packageHistory = useInfiniteQuery({
    queryKey: ["contest-package-history", replaceTarget?.problemId],
    enabled: !!replaceTarget,
    initialPageParam: 1,
    queryFn: ({pageParam}) => unwrap(api.GET("/problems/{id}/packages", {
      params: {path: {id: replaceTarget!.problemId}, query: {page: pageParam, pageSize: 50}},
    })),
    getNextPageParam: (last) => last.pagination.page < last.pagination.total_pages
      ? last.pagination.page + 1 : undefined,
  });
  const loadingPackages = !!replaceTarget && packageHistory.isPending;
  const availablePackages = [...new Map((packageHistory.data?.pages.flatMap((p) => p.packages) ?? [])
    .filter((p) => p.status === "ready" && p.id)
    .map((p) => [p.id!, {value: p.id!, label: `Версия v${p.version ?? "?"} (${new Date(p.created_at || "").toLocaleDateString("ru-RU")})`}])).values()];

  const effectivePackageId = selectedPackageId ?? availablePackages[0]?.value ?? null;

  const openReplacePackageModal = (problemId: string, title: string) => {
    setReplaceTarget({problemId, title});
    setSelectedPackageId(null);
  };

  const handleConfirmReplacePackage = async () => {
    if (!replaceTarget || !effectivePackageId) {
      return;
    }

    setReplacing(true);
    const {error} = await api.POST(
      "/organizations/{org_login}/contests/{contest_login}/problems",
      {
        params: {
          path: {org_login: orgLogin, contest_login: contestLogin},
          query: {
            problem_id: replaceTarget.problemId,
            package_id: effectivePackageId,
          },
        },
      },
    );
    setReplacing(false);

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось обновить пакет задачи",
        color: "red",
      });
      return;
    }

    notifications.show({
      title: "Успешно",
      message: "Пакет задачи обновлён на контесте",
      color: "green",
    });

    setReplaceTarget(null);
    void queryClient.invalidateQueries({queryKey: contestQueryKey});
    void revalidator.revalidate();
  };

  const autocompleteData = searchResults.map((p) => ({
    value: p.id,
    label: p.title,
  }));

  return (
    <>
      <Stack gap="md">
        {/* Add Problem Form */}
        <Stack gap="xs">
          <Text size="sm" fw={500}>
            Добавить задачу
          </Text>
          <Group gap="sm">
            <Autocomplete
              placeholder="Поиск среди ваших задач..."
              value={searchQuery}
              onChange={setSearchQuery}
              onOptionSubmit={(value) => {
                setSelectedProblemId(value);
                const selected = searchResults.find((p) => p.id === value);
                if (selected) {
                  setSearchQuery(selected.title);
                }
              }}
              data={autocompleteData}
              rightSection={searching && <Loader size="xs" />}
              style={{flex: 1}}
            />
            <Button
              onClick={handleAddProblem}
              disabled={!selectedProblemId || adding}
              loading={adding}
              leftSection={<IconPlus size={16} />}
            >
              Добавить
            </Button>
          </Group>
        </Stack>

        <Divider />

        {/* Problems Header & Controls */}
        <Group justify="space-between" align="center" wrap="wrap" gap="sm">
          <Text size="sm" fw={500}>
            Задачи контеста {problems.length > 0 && `(${problems.length})`}
          </Text>
          <Group gap="xs">
            <DownloadStatementsButton
              orgLogin={orgLogin}
              contestLogin={contestLogin}
              size="xs"
              variant="default"
            />
            <Button
              variant="default"
              size="xs"
              leftSection={<IconRefresh size={14} />}
              onClick={handleRecalculatePositions}
              disabled={problems.length === 0}
            >
              Пересчитать позиции
            </Button>
            {hasUnsavedChanges && (
              <>
                <Button
                  variant="subtle"
                  color="gray"
                  size="xs"
                  leftSection={<IconRotate2 size={14} />}
                  onClick={handleResetOrder}
                  disabled={savingOrder}
                >
                  Сбросить
                </Button>
                <Button
                  color="blue"
                  size="xs"
                  leftSection={<IconDeviceFloppy size={14} />}
                  onClick={handleSaveOrder}
                  loading={savingOrder}
                >
                  Сохранить порядок
                </Button>
              </>
            )}
          </Group>
        </Group>

        {/* Problems List */}
        {problems.length === 0 ? (
          <Center py="xl">
            <Stack align="center" gap="sm">
              <Text size="lg" c="dimmed">
                Нет задач в контесте
              </Text>
              <Text size="sm" c="dimmed">
                Добавьте задачи из вашего списка
              </Text>
            </Stack>
          </Center>
        ) : (
          <Table highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th style={{width: 36}} />
                <Table.Th style={{width: 60}}>№</Table.Th>
                <Table.Th>Название</Table.Th>
                <Table.Th style={{width: 120}}>Время</Table.Th>
                <Table.Th style={{width: 120}}>Память</Table.Th>
                <Table.Th style={{width: 130}}>Действия</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {problems.map((problem, index) => (
                <Table.Tr
                  key={problem.problem_id}
                  draggable
                  onDragStart={(e) => handleDragStart(e, index)}
                  onDragOver={(e) => handleDragOver(e, index)}
                  onDragEnd={handleDragEnd}
                  onDrop={(e) => handleDrop(e, index)}
                  className={`
                    ${draggedIndex === index ? classes.rowDragging : ""}
                    ${dragOverIndex === index && draggedIndex !== index ? classes.rowDragOver : ""}
                  `}
                >
                  <Table.Td style={{width: 36, paddingLeft: 8, paddingRight: 4}}>
                    <Tooltip label="Перетащите для изменения порядка" withArrow>
                      <div className={classes.dragHandle}>
                        <IconGripVertical size={16} />
                      </div>
                    </Tooltip>
                  </Table.Td>
                  <Table.Td>
                    <Badge variant="light" color="blue">
                      {numberToLetters(problem.position)}
                    </Badge>
                  </Table.Td>
                  <Table.Td>
                    <Text size="sm" fw={500}>
                      {problem.title}
                    </Text>
                    <Text size="xs" c="dimmed">
                      {problem.problem_id?.toString().slice(0, 8)}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Text size="sm">{problem.time_limit}ms</Text>
                  </Table.Td>
                  <Table.Td>
                    <Text size="sm">{problem.memory_limit}MB</Text>
                  </Table.Td>
                  <Table.Td>
                    <Group gap="xs" wrap="nowrap">
                      <Tooltip label="Редактировать задачу" withArrow>
                        <ActionIcon
                          color="gray"
                          variant="subtle"
                          component="a"
                          href={orgLogin ? `/${orgLogin}/problems/${problem.problem_id}` : `/problems/${problem.problem_id}`}
                          target="_blank"
                          rel="noopener noreferrer"
                        >
                          <IconEdit size={16} />
                        </ActionIcon>
                      </Tooltip>
                      <Tooltip label="Заменить пакет" withArrow>
                        <ActionIcon
                          aria-label={`Заменить пакет ${problem.title}`}
                          color="blue"
                          variant="subtle"
                          onClick={() =>
                            openReplacePackageModal(
                              problem.problem_id,
                              problem.title,
                            )
                          }
                        >
                          <IconRefresh size={16} />
                        </ActionIcon>
                      </Tooltip>
                      <Tooltip label="Удалить задачу" withArrow>
                        <ActionIcon
                          color="red"
                          variant="subtle"
                          onClick={() => handleDeleteProblem(problem.problem_id)}
                          loading={deletingId === problem.problem_id}
                        >
                          <IconTrash size={16} />
                        </ActionIcon>
                      </Tooltip>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Stack>

      <Modal
        opened={!!replaceTarget}
        onClose={() => setReplaceTarget(null)}
        title={`Заменить пакет — ${replaceTarget?.title ?? ""}`}
        centered
        size="sm"
      >
        <Stack gap="md">
          {loadingPackages && (
            <Center py="md">
              <Loader size="sm" />
            </Center>
          )}
          {packageHistory.error && <Text c="red">Не удалось загрузить пакеты задачи: {packageHistory.error.message}</Text>}
          {!loadingPackages && !packageHistory.error && availablePackages.length === 0 && (
            <Text size="sm" c="dimmed">
              Среди загруженных версий нет готовых пакетов.
            </Text>
          )}
          {!loadingPackages && availablePackages.length > 0 && (
            <>
              <Select
                label="Выберите версию пакета"
                data={availablePackages}
                value={effectivePackageId}
                onChange={setSelectedPackageId}
              />
              <Button
                loading={replacing}
                onClick={handleConfirmReplacePackage}
                disabled={!effectivePackageId}
              >
                Обновить пакет на контесте
              </Button>
            </>
          )}
          {(packageHistory.hasNextPage || packageHistory.error) && (
            <Button variant="light" loading={packageHistory.isFetchingNextPage || packageHistory.isFetching}
              onClick={() => packageHistory.error && !packageHistory.data
                ? void packageHistory.refetch() : void packageHistory.fetchNextPage()}>
              {packageHistory.error ? "Повторить загрузку" : "Ещё"}
            </Button>
          )}
        </Stack>
      </Modal>

      <StatusMessage
        type={statusMessage?.type || "success"}
        message={statusMessage?.message || ""}
        opened={!!statusMessage}
        onClose={() => setStatusMessage(null)}
      />
    </>
  );
};
