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

const CONTEST_TEAM_ROLE_OPTIONS = [
  {label: "Участник", value: "participant", color: "gray"},
  {label: "Модератор", value: "moderator", color: "yellow"},
];

const getRoleDisplay = (role: string) => {
  const found = CONTEST_TEAM_ROLE_OPTIONS.find((r) => r.value === role);
  return found || {label: role, color: "gray"};
};

export interface ContestTeamsManagementProps {
  orgLogin: string;
  contestLogin: string;
  contestId?: string;
  orgId?: string;
}

export const ContestTeamsManagement = ({
  orgLogin,
  contestLogin,
  orgId,
}: ContestTeamsManagementProps): ReactNode => {
  const [teams, setTeams] = useState<corev1.ContestTeamModel[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const [editingTeam, setEditingTeam] = useState<{
    teamName: string;
    teamId: string;
    currentRole: string;
  } | null>(null);
  const [modalOpened, setModalOpened] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  const loadTeams = useCallback(async () => {
    setLoading(true);
    const [error, response] = await api.listContestTeams({orgLogin, contestLogin});
    setLoading(false);

    if (error || !response) {
      console.error("Failed to load contest teams:", error);
      return;
    }

    setTeams(response.teams);
  }, [orgLogin, contestLogin]);

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
    const [error] = await api.createContestTeam({
      orgLogin,
      contestLogin,
      teamId: selectedTeamId,
      role: "participant",
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

  const handleDeleteTeam = async (team: corev1.ContestTeamModel) => {
    setDeletingId(team.team_id);
    const [error] = await api.deleteContestTeam({orgLogin, contestLogin, teamId: team.team_id});
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

  const handleEditRole = (team: corev1.ContestTeamModel) => {
    setEditingTeam({
      teamName: team.team_name,
      teamId: team.team_id,
      currentRole: team.contest_role,
    });
    setModalOpened(true);
  };

  const handleChangeRole = async (newRole: string) => {
    if (!editingTeam) {
      return;
    }

    const [error] = await api.updateContestTeam({
      orgLogin,
      contestLogin,
      teamId: editingTeam.teamId,
      role: newRole,
    });

    setModalOpened(false);

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось изменить роль команды",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось изменить роль команды",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Роль команды обновлена",
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

        <EntityManagementTable<corev1.ContestTeamModel>
          items={teams}
          loading={loading}
          getItemKey={(t) => t.team_id}
          emptyMessage="Нет команд. Добавьте команды для привязки к контесту"
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
              header: "Роль",
              align: "center",
              render: (t) => {
                const role = getRoleDisplay(t.contest_role);
                return (
                  <Badge variant="filled" color={role.color} tt="none" size="md">
                    {role.label}
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
          currentRole={editingTeam.currentRole}
          roleOptions={CONTEST_TEAM_ROLE_OPTIONS}
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
