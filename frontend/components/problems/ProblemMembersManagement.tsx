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
import {IconPlus, IconUser} from "@tabler/icons-react";
import {useCallback, useEffect, useState, type ReactNode} from "react";

import {ChangeRoleModal} from "@/components/shared/ChangeRoleModal";
import {EntityManagementTable} from "@/components/shared/EntityManagementTable";
import {StatusMessage} from "@/components/shared/StatusMessage";
import {useEntitySearch} from "@/hooks/useEntitySearch";
import {api} from "@/lib/api";

import type * as corev1 from "@/contracts/core/v1";

const PROBLEM_ROLE_OPTIONS = [
  {label: "Просмотр", value: "viewer", color: "gray"},
  {label: "Редактор", value: "moderator", color: "yellow"},
  {label: "Владелец", value: "owner", color: "red"},
];

const getRoleDisplay = (role: string) => {
  const found = PROBLEM_ROLE_OPTIONS.find((r) => r.value === role);
  return found || {label: role, color: "gray"};
};

export interface ProblemMembersManagementProps {
  problemId: string;
}

export const ProblemMembersManagement = ({problemId}: ProblemMembersManagementProps): ReactNode => {
  const [members, setMembers] = useState<corev1.ProblemMemberModel[]>([]);
  const [loading, setLoading] = useState(true);
  const [adding, setAdding] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const [editingMember, setEditingMember] = useState<{
    username: string;
    userId: string;
    currentRole: string;
  } | null>(null);
  const [modalOpened, setModalOpened] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  const loadMembers = useCallback(async () => {
    setLoading(true);
    const [error, response] = await api.listProblemMembers({id: problemId, page: 1, pageSize: 100});
    setLoading(false);

    if (error || !response) {
      console.error("Failed to load problem members:", error);
      return;
    }

    setMembers(response.members);
  }, [problemId]);

  useEffect(() => {
    loadMembers();
  }, [loadMembers]);

  const searchUsersFn = useCallback(async (query: string) => {
    const [error, response] = await api.listUsers({page: 1, pageSize: 10, search: query});
    if (error || !response) {
      return [];
    }
    return response.users;
  }, []);

  const mapUserToItem = useCallback(
    (u: corev1.UserModel) => ({value: u.id, label: u.username, data: u}),
    []
  );

  const {
    searchQuery,
    setSearchQuery,
    results: searchResults,
    rawResults,
    searching,
    selectedId: selectedUserId,
    selectOption: setSelectedUserId,
    reset: resetSearch,
  } = useEntitySearch<corev1.UserModel>({
    searchFn: searchUsersFn,
    mapToItem: mapUserToItem,
  });

  const handleAddMember = async () => {
    if (!selectedUserId) {
      return;
    }

    setAdding(true);
    const [error] = await api.createProblemMember({
      id: problemId,
      userId: selectedUserId,
      role: "viewer",
    });
    setAdding(false);

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось добавить участника",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось добавить участника",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Участник добавлен",
    });

    resetSearch();
    await loadMembers();
  };

  const handleDeleteMember = async (member: corev1.ProblemMemberModel) => {
    setDeletingId(member.user_id);
    const [error] = await api.deleteProblemMember({id: problemId, userId: member.user_id});
    setDeletingId(null);

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось удалить участника",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось удалить участника",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Участник удален",
    });

    await loadMembers();
  };

  const handleEditRole = (member: corev1.ProblemMemberModel) => {
    setEditingMember({
      username: member.username,
      userId: member.user_id,
      currentRole: member.role,
    });
    setModalOpened(true);
  };

  const handleChangeRole = async (newRole: string) => {
    if (!editingMember) {
      return;
    }

    const [error] = await api.updateProblemMember({
      id: problemId,
      userId: editingMember.userId,
      role: newRole,
    });

    setModalOpened(false);

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось изменить роль участника",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось изменить роль участника",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Роль участника обновлена",
    });

    await loadMembers();
  };

  return (
    <>
      <Stack gap="md">
        <Card withBorder padding="md">
          <Stack gap="sm">
            <Text size="sm" fw={500}>
              Добавить участника
            </Text>
            <Group gap="sm">
              <Autocomplete
                placeholder="Поиск пользователя по username..."
                value={searchQuery}
                onChange={setSearchQuery}
                onOptionSubmit={(value) => {
                  setSelectedUserId(value);
                  const selected = rawResults.find((u) => u.id === value);
                  if (selected) {
                    setSearchQuery(selected.username);
                  }
                }}
                data={searchResults}
                rightSection={searching && <Loader size="xs" />}
                style={{flex: 1}}
              />
              <Button
                onClick={handleAddMember}
                disabled={!selectedUserId || adding}
                loading={adding}
                leftSection={<IconPlus size={16} />}
              >
                Добавить
              </Button>
            </Group>
          </Stack>
        </Card>

        <EntityManagementTable<corev1.ProblemMemberModel>
          items={members}
          loading={loading}
          getItemKey={(m) => m.user_id}
          emptyMessage="Нет участников. Добавьте участников для совместной работы над задачей"
          emptyIcon={<IconUser size={32} color="var(--mantine-color-dimmed)" />}
          columns={[
            {
              header: "Пользователь",
              render: (m) => (
                <Text size="sm" fw={500}>
                  {m.username}
                </Text>
              ),
            },
            {
              header: "Роль",
              align: "center",
              render: (m) => {
                const role = getRoleDisplay(m.role);
                return (
                  <Badge variant="filled" color={role.color} tt="none" size="md">
                    {role.label}
                  </Badge>
                );
              },
            },
          ]}
          onEdit={handleEditRole}
          onDelete={handleDeleteMember}
          deletingId={deletingId}
        />
      </Stack>

      {editingMember && (
        <ChangeRoleModal
          opened={modalOpened}
          onClose={() => setModalOpened(false)}
          participant={{
            username: editingMember.username,
            userId: editingMember.userId,
          }}
          currentRole={editingMember.currentRole}
          roleOptions={PROBLEM_ROLE_OPTIONS}
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
