"use client";

import {
  ActionIcon,
  Autocomplete,
  Badge,
  Button,
  Card,
  Center,
  Group,
  Loader,
  Modal,
  Select,
  Stack,
  Table,
  Text,
  Tooltip,
} from "@mantine/core";
import {useDebouncedValue} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {IconEdit, IconPlus, IconRefresh, IconTrash} from "@tabler/icons-react";
import {useRouter} from "next/navigation";
import {useEffect, useState} from "react";

import {StatusMessage} from "@/components/shared/StatusMessage";
import {api} from "@/lib/api";
import {numberToLetters} from "@/lib/lib";

import type * as corev1 from "@/contracts/core/v1";
import type {ReactNode} from "react";

interface ProblemsSectionProps {
  contestId: string;
  initialProblems: Array<corev1.ContestProblemListItemModel>;
  orgLogin?: string;
}

export const ProblemsSection = ({
  contestId,
  initialProblems,
  orgLogin,
}: ProblemsSectionProps): ReactNode => {
  const router = useRouter();
  const [problems, setProblems] = useState(initialProblems);
  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedQuery] = useDebouncedValue(searchQuery, 300);
  const [searchResults, setSearchResults] = useState<corev1.ProblemsListItemModel[]>([]);
  const [searching, setSearching] = useState(false);
  const [selectedProblemId, setSelectedProblemId] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  // Modal for replacing package version
  const [replaceTarget, setReplaceTarget] = useState<{ problemId: string; title: string } | null>(null);
  const [availablePackages, setAvailablePackages] = useState<Array<{ value: string; label: string }>>([]);
  const [selectedPackageId, setSelectedPackageId] = useState<string | null>(null);
  const [loadingPackages, setLoadingPackages] = useState(false);
  const [replacing, setReplacing] = useState(false);

  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  useEffect(() => {
    setProblems(initialProblems);
  }, [initialProblems]);

  useEffect(() => {
    const searchProblemsAsync = async () => {
      if (!debouncedQuery || debouncedQuery.length < 2) {
        setSearchResults([]);
        return;
      }

      setSearching(true);
      const [error, response] = await api.listProblems({page: 1, pageSize: 10, search: debouncedQuery, owner: true});
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
    const [error] = await api.createContestProblem({contestId, problemId: selectedProblemId});
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
    router.refresh();
  };

  const handleDeleteProblem = async (problemId: string) => {
    setDeletingId(problemId);
    const [error] = await api.deleteContestProblem({contestId, problemId});
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

    router.refresh();
  };

  const openReplacePackageModal = async (problemId: string, title: string) => {
    setReplaceTarget({problemId, title});
    setSelectedPackageId(null);
    setLoadingPackages(true);

    const [error, response] = await api.listProblemPackages({id: problemId});
    setLoadingPackages(false);

    if (error || !response?.packages) {
      notifications.show({
        title: "Ошибка",
        message: error?.message || "Не удалось загрузить пакеты задачи",
        color: "red",
      });
      return;
    }

    const readyPkgs = response.packages
      .filter((p) => p.status === "ready" && p.id)
      .map((p) => ({
        value: p.id!,
        label: `Версия v${p.version ?? "?"} (${new Date(p.created_at || "").toLocaleDateString("ru-RU")})`,
      }));

    setAvailablePackages(readyPkgs);
    if (readyPkgs.length > 0) {
      setSelectedPackageId(readyPkgs[0].value);
    }
  };

  const handleConfirmReplacePackage = async () => {
    if (!replaceTarget || !selectedPackageId) {
      return;
    }

    setReplacing(true);
    const [error] = await api.createContestProblem({
      contestId,
      problemId: replaceTarget.problemId,
      packageId: selectedPackageId,
    });
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
    router.refresh();
  };

  const autocompleteData = searchResults.map((p) => ({
    value: p.id,
    label: p.title,
  }));

  return (
    <>
      <Stack gap="md">
        {/* Add Problem Form */}
        <Card withBorder padding="md">
          <Stack gap="sm">
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
        </Card>

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
                <Table.Th style={{width: 60}}>№</Table.Th>
                <Table.Th>Название</Table.Th>
                <Table.Th style={{width: 120}}>Время</Table.Th>
                <Table.Th style={{width: 120}}>Память</Table.Th>
                <Table.Th style={{width: 130}}>Действия</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {problems.map((problem) => (
                <Table.Tr key={problem.problem_id}>
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
          {!loadingPackages && availablePackages.length === 0 && (
            <Text size="sm" c="dimmed">
              У этой задачи пока нет скомпилированных пакетов.
            </Text>
          )}
          {!loadingPackages && availablePackages.length > 0 && (
            <>
              <Select
                label="Выберите версию пакета"
                data={availablePackages}
                value={selectedPackageId}
                onChange={setSelectedPackageId}
              />
              <Button
                loading={replacing}
                onClick={handleConfirmReplacePackage}
                disabled={!selectedPackageId}
              >
                Обновить пакет на контесте
              </Button>
            </>
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
