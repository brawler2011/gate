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
  Stack,
  Table,
  Text,
} from "@mantine/core";
import {useDebouncedValue} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {IconEdit, IconPlus, IconTrash, IconUsersGroup} from "@tabler/icons-react";
import {useCallback, useEffect, useState} from "react";

import {StatusMessage} from "@/components/shared/StatusMessage";
import {ChangeRoleModal} from "@/components/contests/ChangeRoleModal";
import {api} from "@/lib/api";

import type * as corev1 from "@/contracts/core/v1";
import type {ReactNode} from "react";

const PROBLEM_PERMISSION_OPTIONS = [
  {label: "Чтение (read)", value: "read", color: "gray"},
  {label: "Запись (write)", value: "write", color: "yellow"},
  {label: "Админ (admin)", value: "admin", color: "red"},
];

const getPermissionDisplay = (perm: string) => {
  const found = PROBLEM_PERMISSION_OPTIONS.find((r) => r.value === perm);
  return found || {label: perm, color: "gray"};
};

interface ProblemTeamsManagementProps {
  problemId: string;
  orgId?: string;
}

export const ProblemTeamsManagement = ({problemId, orgId}: ProblemTeamsManagementProps): ReactNode => {
  const [teams, setTeams] = useState<corev1.ProblemTeamModel[]>([]);
  const [loading, setLoading] = useState(true);

  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedQuery] = useDebouncedValue(searchQuery, 300);
  const [searchResults, setSearchResults] = useState<corev1.TeamModel[]>([]);
  const [searching, setSearching] = useState(false);
  const [selectedTeamId, setSelectedTeamId] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const [editingTeam, setEditingTeam] = useState<{
    teamName: string;
    teamId: string;
    currentPermission: string;
  } | null>(null);
  const [modalOpened, setModalOpened] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  const loadTeams = useCallback(async () => {
    setLoading(true);
    const [error, response] = await api.listProblemTeams({id: problemId});
    setLoading(false);

    if (error || !response) {
      console.error("Failed to load problem teams:", error);
      return;
    }

    setTeams(response.teams);
  }, [problemId]);

  useEffect(() => {
    loadTeams();
  }, [loadTeams]);

  useEffect(() => {
    const searchTeamsAsync = async () => {
      if (!debouncedQuery || debouncedQuery.length < 2) {
        setSearchResults([]);
        return;
      }

      setSearching(true);
      const [error, response] = await api.listTeams({
        organizationId: orgId,
        search: debouncedQuery,
        page: 1,
        pageSize: 10,
      });
      setSearching(false);

      if (error || !response) {
        return;
      }

      setSearchResults(response.teams);
    };

    searchTeamsAsync();
  }, [debouncedQuery, orgId]);

  const handleAddTeam = async () => {
    if (!selectedTeamId) {
      return;
    }

    setAdding(true);
    const [error] = await api.createProblemTeam({
      id: problemId,
      teamId: selectedTeamId,
      permission: "read",
    });
    setAdding(false);

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось добавить команду",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось добавить команду",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Команда добавлена",
    });

    setSearchQuery("");
    setSelectedTeamId(null);
    await loadTeams();
  };

  const handleDeleteTeam = async (teamId: string) => {
    setDeletingId(teamId);
    const [error] = await api.deleteProblemTeam({id: problemId, teamId});
    setDeletingId(null);

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось отозвать доступ у команды",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось отозвать доступ у команды",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Доступ команды отозван",
    });

    await loadTeams();
  };

  const handleEditPermission = (team: corev1.ProblemTeamModel) => {
    setEditingTeam({
      teamName: team.team_name,
      teamId: team.team_id,
      currentPermission: team.permission,
    });
    setModalOpened(true);
  };

  const handleChangePermission = async (newPermission: string) => {
    if (!editingTeam) {
      return;
    }

    const [error] = await api.updateProblemTeam({
      id: problemId,
      teamId: editingTeam.teamId,
      permission: newPermission,
    });

    setModalOpened(false);

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось изменить права команды",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось изменить права команды",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Права команды обновлены",
    });

    await loadTeams();
  };

  const autocompleteData = searchResults.map((t) => ({
    value: t.id,
    label: t.name,
  }));

  return (
    <>
      <Stack gap="md">
        <Card withBorder padding="md">
          <Stack gap="sm">
            <Text size="sm" fw={500}>
              Выдать доступ команде
            </Text>
            <Group gap="sm">
              <Autocomplete
                placeholder="Поиск команды по названию..."
                value={searchQuery}
                onChange={setSearchQuery}
                onOptionSubmit={(value) => {
                  setSelectedTeamId(value);
                  const selected = searchResults.find((t) => t.id === value);
                  if (selected) {
                    setSearchQuery(selected.name);
                  }
                }}
                data={autocompleteData}
                rightSection={searching && <Loader size="xs" />}
                style={{flex: 1}}
              />
              <Button
                onClick={handleAddTeam}
                disabled={!selectedTeamId || adding}
                loading={adding}
                leftSection={<IconPlus size={16} />}
              >
                Добавить
              </Button>
            </Group>
          </Stack>
        </Card>

        {loading && (
          <Center py="xl">
            <Loader size="md" />
          </Center>
        )}
        {!loading && teams.length === 0 && (
          <Center py="xl">
            <Stack align="center" gap="sm">
              <IconUsersGroup size={32} color="var(--mantine-color-dimmed)" />
              <Text size="lg" c="dimmed">
                Нет команд с доступом
              </Text>
              <Text size="sm" c="dimmed">
                Добавьте команды для совместной работы над задачей
              </Text>
            </Stack>
          </Center>
        )}
        {!loading && teams.length > 0 && (
          <Table highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th style={{width: 180}}>Команда</Table.Th>
                <Table.Th style={{textAlign: "center"}}>Права доступа</Table.Th>
                <Table.Th style={{width: 80}}>Действия</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {teams.map((t) => (
                <Table.Tr key={t.team_id}>
                  <Table.Td>
                    <Text size="sm" fw={500}>
                      {t.team_name}
                    </Text>
                  </Table.Td>
                  <Table.Td style={{textAlign: "center"}}>
                    <Badge
                      variant="filled"
                      color={getPermissionDisplay(t.permission).color}
                      tt="none"
                      size="md"
                    >
                      {getPermissionDisplay(t.permission).label}
                    </Badge>
                  </Table.Td>
                  <Table.Td>
                    <Group gap="xs" wrap="nowrap">
                      <ActionIcon
                        color="blue"
                        variant="subtle"
                        onClick={() => handleEditPermission(t)}
                      >
                        <IconEdit size={16} />
                      </ActionIcon>
                      <ActionIcon
                        color="red"
                        variant="subtle"
                        onClick={() => handleDeleteTeam(t.team_id)}
                        loading={deletingId === t.team_id}
                      >
                        <IconTrash size={16} />
                      </ActionIcon>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        )}
      </Stack>

      {editingTeam && (
        <ChangeRoleModal
          opened={modalOpened}
          onClose={() => setModalOpened(false)}
          participant={{
            username: editingTeam.teamName,
            userId: editingTeam.teamId,
          }}
          currentRole={editingTeam.currentPermission}
          onSubmit={handleChangePermission}
        />
      )}

      <StatusMessage
        type={statusMessage?.type || "success"}
        message={statusMessage?.message || ""}
        opened={!!statusMessage}
        onClose={() => setStatusMessage(null)}
      />
    </>
  );
};
