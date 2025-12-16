"use client";

import {
  addContestMember,
  getContestMembers,
  removeContestMember,
  searchUsers,
  updateContestMemberRole,
} from "@/lib/actions";
import {
  ActionIcon,
  Autocomplete,
  Badge,
  Button,
  Card,
  Center,
  Group,
  Loader,
  Pagination,
  Stack,
  Table,
  Text,
} from "@mantine/core";
import { useDebouncedValue } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconEdit, IconPlus, IconTrash } from "@tabler/icons-react";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import type * as corev1 from "@contracts/core/v1";
import { ChangeRoleModal } from "./ChangeRoleModal";
import { StatusMessage } from "@/components/StatusMessage";

const ROLE_OPTIONS = [
  { label: "Участник", value: "participant", color: "gray" },
  { label: "Модератор", value: "moderator", color: "yellow" },
  { label: "Создатель", value: "owner", color: "red" },
];

function getRoleDisplay(role: string) {
  const roleOption = ROLE_OPTIONS.find(r => r.value === role);
  return roleOption || { label: role, color: "gray" };
}

interface ParticipantsSectionProps {
  contestId: string;
}

export function ParticipantsSection({ contestId }: ParticipantsSectionProps) {
  const router = useRouter();
  const [participants, setParticipants] = useState<corev1.ContestMemberModel[]>([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [loading, setLoading] = useState(true);

  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedQuery] = useDebouncedValue(searchQuery, 300);
  const [searchResults, setSearchResults] = useState<corev1.UserModel[]>([]);
  const [searching, setSearching] = useState(false);
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const [editingParticipant, setEditingParticipant] = useState<{
    username: string;
    userId: string;
    currentRole: string;
  } | null>(null);
  const [modalOpened, setModalOpened] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  const pageSize = 10;

  // Load participants
  const loadParticipants = async () => {
    setLoading(true);
    const [error, response] = await getContestMembers(contestId, page, pageSize);
    setLoading(false);

    if (error || !response) {
      console.error("Failed to load participants:", error);
      return;
    }

    setParticipants(response.members);
    const total = response?.pagination?.total || 0;
    setTotalPages(Math.ceil(total / pageSize));
  };

  // Load participants on mount and when page changes
  useEffect(() => {
    loadParticipants();
  }, [contestId, page]);

  // Search for users
  useEffect(() => {
    const searchUsersAsync = async () => {
      if (!debouncedQuery || debouncedQuery.length < 2) {
        setSearchResults([]);
        return;
      }

      setSearching(true);
      const [error, response] = await searchUsers(debouncedQuery);
      setSearching(false);

      if (error || !response) {
        console.error("Failed to search users:", error);
        return;
      }

      setSearchResults(response.users);
    };

    searchUsersAsync();
  }, [debouncedQuery]);

  const handleAddParticipant = async () => {
    if (!selectedUserId) return;

    setAdding(true);
    const [error] = await addContestMember(contestId, selectedUserId);
    setAdding(false);

    if (error) {
      console.error("Failed to add participant:", error);
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

    setSearchQuery("");
    setSelectedUserId(null);
    await loadParticipants();
  };

  const handleDeleteParticipant = async (userId: string) => {
    setDeletingId(userId);
    const [error] = await removeContestMember(contestId, userId);
    setDeletingId(null);

    if (error) {
      console.error("Failed to delete participant:", error);
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

    await loadParticipants();
  };

  const handleEditRole = (user: corev1.ContestMemberModel) => {
    setEditingParticipant({
      username: user.username,
      userId: user.user_id,
      currentRole: user.contest_role,
    });
    setModalOpened(true);
  };

  const handleChangeRole = async (newRole: string) => {
    if (!editingParticipant) return;

    const [error] = await updateContestMemberRole(
      contestId,
      editingParticipant.userId,
      newRole
    );

    setModalOpened(false);

    if (error) {
      console.error("Failed to change role:", error);
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
      message: "Роль обновлена успешно",
    });

    await loadParticipants();
  };

  const autocompleteData = searchResults.map((u) => ({
    value: u.id,
    label: `${u.username} (${u.role})`,
  }));

  return (
    <>
      <Stack gap="md">
        {/* Add Participant Form */}
        <Card withBorder padding="md">
          <Stack gap="sm">
            <Text size="sm" fw={500}>
              Добавить участника
            </Text>
            <Group gap="sm">
              <Autocomplete
                placeholder="Поиск по имени пользователя или email..."
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
                style={{ flex: 1 }}
              />
              <Button
                onClick={handleAddParticipant}
                disabled={!selectedUserId || adding}
                loading={adding}
                leftSection={<IconPlus size={16} />}
              >
                Добавить
              </Button>
            </Group>
          </Stack>
        </Card>

        {/* Participants List */}
        {loading ? (
          <Center py="xl">
            <Loader size="md" />
          </Center>
        ) : participants.length === 0 ? (
          <Center py="xl">
            <Stack align="center" gap="sm">
              <Text size="lg" c="dimmed">
                Нет участников
              </Text>
              <Text size="sm" c="dimmed">
                Добавьте участников для контеста
              </Text>
            </Stack>
          </Center>
        ) : (
          <Stack gap="md">
            <Table highlightOnHover>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th style={{ width: 140 }}>Пользователь</Table.Th>
                  <Table.Th style={{ textAlign: 'center' }}>Роль</Table.Th>
                  <Table.Th style={{ width: 80 }}>Действия</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {participants.map((user) => (
                  <Table.Tr key={user.user_id}>
                    <Table.Td>
                      <Text size="sm" fw={500}>
                        {user.username}
                      </Text>
                    </Table.Td>
                    <Table.Td style={{ textAlign: 'center' }}>
                      <Badge
                        variant="filled"
                        color={getRoleDisplay(user.contest_role).color}
                        tt="none"
                        size="md"
                      >
                        {getRoleDisplay(user.contest_role).label}
                      </Badge>
                    </Table.Td>
                    <Table.Td>
                      <Group gap="xs" wrap="nowrap">
                        {user.contest_role !== "owner" ? (
                          <ActionIcon
                            color="blue"
                            variant="subtle"
                            onClick={() => handleEditRole(user)}
                          >
                            <IconEdit size={16} />
                          </ActionIcon>
                        ) : (
                          <div style={{ width: 28 }} />
                        )}
                        <ActionIcon
                          color="red"
                          variant="subtle"
                          onClick={() => handleDeleteParticipant(user.user_id)}
                          loading={deletingId === user.user_id}
                        >
                          <IconTrash size={16} />
                        </ActionIcon>
                      </Group>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>

            {totalPages > 1 && (
              <Center>
                <Pagination
                  value={page}
                  onChange={setPage}
                  total={totalPages}
                />
              </Center>
            )}
          </Stack>
        )}
      </Stack>

      {editingParticipant && (
        <ChangeRoleModal
          opened={modalOpened}
          onClose={() => setModalOpened(false)}
          participant={{
            username: editingParticipant.username,
            userId: editingParticipant.userId,
          }}
          currentRole={editingParticipant.currentRole}
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
}
