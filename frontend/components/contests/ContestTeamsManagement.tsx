"use client";

import {Autocomplete, Badge, Button, Card, Group, Loader, Stack, Text} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconPlus, IconUsersGroup} from "@tabler/icons-react";
import {useCallback, useState, type ReactNode} from "react";

import {ChangeRoleModal} from "@/components/shared/ChangeRoleModal";
import {EntityManagementTable} from "@/components/shared/EntityManagementTable";
import {StatusMessage} from "@/components/shared/StatusMessage";
import {useEntityCollection, useEntityMutation} from "@/hooks/useEntityManagement";
import {useEntitySearch} from "@/hooks/useEntitySearch";
import {api} from "@/lib/api";

import type * as corev1 from "@/contracts/core/v1/schema";

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
  const {pendingId: addingId, run: addTeam} = useEntityMutation();
  const adding = addingId !== null;
  const {pendingId: deletingId, run: deleteTeam} = useEntityMutation();

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

  const fetchTeams = useCallback(async () => {
    const {data: response, error} = await api.GET(
      "/organizations/{org_login}/contests/{contest_login}/teams",
      {
        params: {path: {org_login: orgLogin, contest_login: contestLogin}},
      },
    );
    if (error || !response) {
      console.error("Failed to load contest teams:", error);
      return;
    }
    return response.teams;
  }, [orgLogin, contestLogin]);
  const {items: teams, loading, reload: loadTeams} = useEntityCollection(fetchTeams);

  const searchTeamsFn = useCallback(
    async (query: string) => {
      const {data: response, error} = await api.GET("/teams", {
        params: {
          query: {
            page: 1,
            pageSize: 10,
            search: query,
            organization_id: orgId,
          },
        },
      });
      if (error || !response) {
        return [];
      }
      return response.teams;
    },
    [orgId],
  );

  const mapTeamToItem = useCallback(
    (t: corev1.TeamModel) => ({value: t.id, label: t.name, data: t}),
    [],
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

    const error = await addTeam(selectedTeamId, async () => {
      const {error} = await api.POST(
        "/organizations/{org_login}/contests/{contest_login}/teams",
        {
          params: {
            path: {org_login: orgLogin, contest_login: contestLogin},
            query: {team_id: selectedTeamId, role: "participant"},
          },
        },
      );
      return error;
    });

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
    const error = await deleteTeam(team.team_id, async () => {
      const {error} = await api.DELETE(
        "/organizations/{org_login}/contests/{contest_login}/teams",
        {
          params: {
            path: {org_login: orgLogin, contest_login: contestLogin},
            query: {team_id: team.team_id},
          },
        },
      );
      return error;
    });

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

    const {error} = await api.PATCH(
      "/organizations/{org_login}/contests/{contest_login}/teams",
      {
        params: {
          path: {org_login: orgLogin, contest_login: contestLogin},
          query: {team_id: editingTeam.teamId, role: newRole},
        },
      },
    );

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
