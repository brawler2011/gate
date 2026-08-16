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
import {IconEdit, IconPlus, IconTrash, IconUser} from "@tabler/icons-react";
import {useCallback, useEffect, useState} from "react";

import {StatusMessage} from "@/components/shared/StatusMessage";
import {ChangeRoleModal} from "@/components/contests/ChangeRoleModal";
import {api} from "@/lib/api";

import type * as corev1 from "@/contracts/core/v1";
import type {ReactNode} from "react";

const PROBLEM_ROLE_OPTIONS = [
  {label: "Просмотр", value: "viewer", color: "gray"},
  {label: "Редактор", value: "moderator", color: "yellow"},
  {label: "Владелец", value: "owner", color: "red"},
];

const getRoleDisplay = (role: string) => {
  const found = PROBLEM_ROLE_OPTIONS.find((r) => r.value === role);
  return found || {label: role, color: "gray"};
};

interface ProblemMembersManagementProps {
  problemId: string;
}

export const ProblemMembersManagement = ({problemId}: ProblemMembersManagementProps): ReactNode => {
  const [members, setMembers] = useState<corev1.ProblemMemberModel[]>([]);
  const [loading, setLoading] = useState(true);

  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedQuery] = useDebouncedValue(searchQuery, 300);
  const [searchResults, setSearchResults] = useState<corev1.UserModel[]>([]);
  const [searching, setSearching] = useState(false);
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);
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

  useEffect(() => {
    const searchUsersAsync = async () => {
      if (!debouncedQuery || debouncedQuery.length < 2) {
        setSearchResults([]);
        return;
      }

      setSearching(true);
      const [error, response] = await api.listUsers({page: 1, pageSize: 10, search: debouncedQuery});
      setSearching(false);

      if (error || !response) {
        return;
      }

      setSearchResults(response.users);
    };

    searchUsersAsync();
  }, [debouncedQuery]);

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
        message: error.message || "Не удалось добавить доступ пользователю",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось добавить доступ пользователю",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Пользователь добавлен",
    });

    setSearchQuery("");
    setSelectedUserId(null);
    await loadMembers();
  };

  const handleDeleteMember = async (userId: string) => {
    setDeletingId(userId);
    const [error] = await api.deleteProblemMember({id: problemId, userId});
    setDeletingId(null);

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось отзывать доступ",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось отозвать доступ",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Доступ отозван",
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
        message: error.message || "Не удалось изменить роль",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось изменить роль",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Роль успешно обновлена",
    });

    await loadMembers();
  };

  const autocompleteData = searchResults.map((u) => ({
    value: u.id,
    label: `${u.username} (${u.role})`,
  }));

  return (
    <>
      <Stack gap="md">
        <Card withBorder padding="md">
          <Stack gap="sm">
            <Text size="sm" fw={500}>
              Выдать доступ пользователю
            </Text>
            <Group gap="sm">
              <Autocomplete
                placeholder="Поиск пользователя..."
                value={searchQuery}
                onChange={setSearchQuery}
                onOptionSubmit={(value) => {
                  setSelectedUserId(value);
                  const selected = searchResults.find((u) => u.id === value);
                  if (selected) {
                    setSearchQuery(selected.username);
                  }
                }}
                data={autocompleteData}
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

        {loading && (
          <Center py="xl">
            <Loader size="md" />
          </Center>
        )}
        {!loading && members.length === 0 && (
          <Center py="xl">
            <Stack align="center" gap="sm">
              <IconUser size={32} color="var(--mantine-color-dimmed)" />
              <Text size="lg" c="dimmed">
                Нет индивидуального доступа
              </Text>
              <Text size="sm" c="dimmed">
                Добавьте пользователей с индивидуальным доступом к задаче
              </Text>
            </Stack>
          </Center>
        )}
        {!loading && members.length > 0 && (
          <Table highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th style={{width: 180}}>Пользователь</Table.Th>
                <Table.Th style={{textAlign: "center"}}>Уровень доступа</Table.Th>
                <Table.Th style={{width: 80}}>Действия</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {members.map((m) => (
                <Table.Tr key={m.user_id}>
                  <Table.Td>
                    <Text size="sm" fw={500}>
                      {m.username}
                    </Text>
                  </Table.Td>
                  <Table.Td style={{textAlign: "center"}}>
                    <Badge
                      variant="filled"
                      color={getRoleDisplay(m.role).color}
                      tt="none"
                      size="md"
                    >
                      {getRoleDisplay(m.role).label}
                    </Badge>
                  </Table.Td>
                  <Table.Td>
                    <Group gap="xs" wrap="nowrap">
                      {m.role !== "owner" ? (
                        <ActionIcon
                          color="blue"
                          variant="subtle"
                          onClick={() => handleEditRole(m)}
                        >
                          <IconEdit size={16} />
                        </ActionIcon>
                      ) : (
                        <div style={{width: 28}} />
                      )}
                      <ActionIcon
                        color="red"
                        variant="subtle"
                        onClick={() => handleDeleteMember(m.user_id)}
                        loading={deletingId === m.user_id}
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

      {editingMember && (
        <ChangeRoleModal
          opened={modalOpened}
          onClose={() => setModalOpened(false)}
          participant={{
            username: editingMember.username,
            userId: editingMember.userId,
          }}
          currentRole={editingMember.currentRole}
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
