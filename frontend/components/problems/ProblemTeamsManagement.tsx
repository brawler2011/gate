"use client";

import {
  Autocomplete,
  Badge,
  Button,
  Card,
  Group,
  Loader,
  Stack,
  Text,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconPlus, IconUsersGroup} from "@tabler/icons-react";
import {useCallback, useEffect, useState, type ReactNode} from "react";

import {ChangeRoleModal} from "@/components/shared/ChangeRoleModal";
import {EntityManagementTable} from "@/components/shared/EntityManagementTable";
import {StatusMessage} from "@/components/shared/StatusMessage";
import {useEntitySearch} from "@/hooks/useEntitySearch";
import {api} from "@/lib/api";

import type * as corev1 from "@/contracts/core/v1";

const PROBLEM_PERMISSION_OPTIONS = [
  {label: "Чтение (read)", value: "read", color: "gray"},
  {label: "Запись (write)", value: "write", color: "yellow"},
  {label: "Админ (admin)", value: "admin", color: "red"},
];

const getPermissionDisplay = (perm: string) => {
  const found = PROBLEM_PERMISSION_OPTIONS.find((r) => r.value === perm);
  return found || {label: perm, color: "gray"};
};

export interface ProblemTeamsManagementProps {
  problemId: string;
  orgId?: string;
}

export const ProblemTeamsManagement = ({problemId, orgId}: ProblemTeamsManagementProps): ReactNode => {
  const [teams, setTeams] = useState<corev1.ProblemTeamModel[]>([]);
  const [loading, setLoading] = useState(true);
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

  const searchTeamsFn = useCallback(
    async (query: string) => {
      const [error, response] = await api.listTeams({
        organizationId: orgId,
        search: query,
        page: 1,
        pageSize: 10,
      });
      if (error || !response) {
        return [];
      }
      return response.teams;
    },
    [orgId]
  );

  const mapTeamToItem = useCallback(
    (t: corev1.TeamModel) => ({value: t.id, label: t.name, data: t}),
    []
  );

  const {
    searchQuery,
    setSearchQuery,
    results: searchResults,
    rawResults,
    searching,
    selectedId: selectedTeamId,
    selectOption: setSelectedTeamId,
    reset: resetSearch,
  } = useEntitySearch<corev1.TeamModel>({
    searchFn: searchTeamsFn,
    mapToItem: mapTeamToItem,
  });

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

    resetSearch();
    await loadTeams();
  };

  const handleDeleteTeam = async (team: corev1.ProblemTeamModel) => {
    setDeletingId(team.team_id);
    const [error] = await api.deleteProblemTeam({id: problemId, teamId: team.team_id});
    setDeletingId(null);

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось удалить команду",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось удалить команду",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Команда удалена",
    });

    await loadTeams();
  };

  const handleEditRole = (team: corev1.ProblemTeamModel) => {
    setEditingTeam({
      teamName: team.team_name,
      teamId: team.team_id,
      currentPermission: team.permission,
    });
    setModalOpened(true);
  };

  const handleChangeRole = async (newRole: string) => {
    if (!editingTeam) {
      return;
    }

    const [error] = await api.updateProblemTeam({
      id: problemId,
      teamId: editingTeam.teamId,
      permission: newRole,
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

  return (
    <>
      <Stack gap="md">
        <Card withBorder padding="md">
          <Stack gap="sm">
            <Text size="sm" fw={500}>
              Добавить команду
            </Text>
            <Group gap="sm">
              <Autocomplete
                placeholder="Поиск команды по названию..."
                value={searchQuery}
                onChange={setSearchQuery}
                onOptionSubmit={(value) => {
                  setSelectedTeamId(value);
                  const selected = rawResults.find((t) => t.id === value);
                  if (selected) {
                    setSearchQuery(selected.name);
                  }
                }}
                data={searchResults}
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

        <EntityManagementTable<corev1.ProblemTeamModel>
          items={teams}
          loading={loading}
          getItemKey={(t) => t.team_id}
          emptyMessage="Нет команд. Добавьте команды для управления правами на задачу"
          emptyIcon={<IconUsersGroup size={32} color="var(--mantine-color-dimmed)" />}
          columns={[
            {
              header: "Команда",
              render: (t) => (
                <Text size="sm" fw={500}>
                  {t.team_name}
                </Text>
              ),
            },
            {
              header: "Права",
              align: "center",
              render: (t) => {
                const perm = getPermissionDisplay(t.permission);
                return (
                  <Badge variant="filled" color={perm.color} tt="none" size="md">
                    {perm.label}
                  </Badge>
                );
              },
            },
          ]}
          onEdit={handleEditRole}
          onDelete={handleDeleteTeam}
          deletingId={deletingId}
        />
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
          roleOptions={PROBLEM_PERMISSION_OPTIONS}
          title="Изменить права"
          selectLabel="Новые права"
          onSubmit={handleChangeRole}
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
