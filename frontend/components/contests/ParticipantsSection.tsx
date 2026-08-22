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
  Pagination,
  Stack,
  Table,
  Tabs,
  Text,
} from "@mantine/core";
import {useDebouncedValue} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {
  IconCheck,
  IconEdit,
  IconPlus,
  IconTrash,
  IconUser,
  IconUserCheck,
  IconUsersGroup,
  IconX,
} from "@tabler/icons-react";
import {useCallback, useEffect, useState} from "react";

import {StatusMessage} from '@/components/shared/StatusMessage';
import {api} from "@/lib/api";

import {ChangeRoleModal} from "./ChangeRoleModal";
import {ContestTeamsManagement} from "./ContestTeamsManagement";

import type * as corev1 from "@/contracts/core/v1";
import type {ReactNode} from "react";

const ROLE_OPTIONS = [
  {label: "Участник", value: "participant", color: "gray"},
  {label: "Модератор", value: "moderator", color: "yellow"},
  {label: "Создатель", value: "owner", color: "red"},
];

const getRoleDisplay = (role: string) => {
  const roleOption = ROLE_OPTIONS.find(r => r.value === role);
  return roleOption || {label: role, color: "gray"};
};

interface ParticipantsSectionProps {
  orgLogin: string;
  contestLogin: string;
  contestId?: string;
  orgId?: string;
}

export const ParticipantsSection = ({
  orgLogin,
  contestLogin,
  contestId,
  orgId,
}: ParticipantsSectionProps): ReactNode => {
  const [activeTab, setActiveTab] = useState<string | null>("users");
  const [participants, setParticipants] = useState<corev1.ContestMemberModel[]>([]);
  const [page, setPage] = useState(1);
  const [totalPages, setTotalPages] = useState(1);
  const [loading, setLoading] = useState(true);

  // Requests state
  const [requests, setRequests] = useState<corev1.ContestJoinRequestModel[]>([]);
  const [loadingRequests, setLoadingRequests] = useState(false);
  const [processingRequestId, setProcessingRequestId] = useState<string | null>(null);

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
  const loadParticipants = useCallback(async () => {
    setLoading(true);
    const [error, response] = await api.listContestMembers({orgLogin, contestLogin, page, pageSize});
    setLoading(false);

    if (error || !response) {
      console.error("Failed to load participants:", error);
      return;
    }

    setParticipants(response.members);
    const total = response?.pagination?.total || 0;
    setTotalPages(Math.ceil(total / pageSize));
  }, [orgLogin, contestLogin, page]);

  // Load requests
  const loadRequests = useCallback(async () => {
    setLoadingRequests(true);
    const [, data] = await api.listContestJoinRequests({orgLogin, contestLogin});
    setLoadingRequests(false);
    if (data) {
      setRequests(data.requests);
    }
  }, [orgLogin, contestLogin]);

  useEffect(() => {
    loadParticipants();
    loadRequests();
  }, [loadParticipants, loadRequests]);

  // Search for users
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
        console.error("Failed to search users:", error);
        return;
      }

      setSearchResults(response.users);
    };

    searchUsersAsync();
  }, [debouncedQuery]);

  const handleAddParticipant = async () => {
    if (!selectedUserId) {
      return;
    }

    setAdding(true);
    const [error] = await api.createContestMember({orgLogin, contestLogin, userId: selectedUserId});
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
    const [error] = await api.deleteContestMember({orgLogin, contestLogin, userId});
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
    if (!editingParticipant) {
      return;
    }

    const [error] = await api.updateContestMember({
      orgLogin,
      contestLogin,
      userId: editingParticipant.userId,
      role: newRole,
    });

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

  const handleApproveRequest = async (requestId: string, username: string) => {
    setProcessingRequestId(requestId);
    const [error] = await api.approveContestJoinRequest({
      orgLogin,
      contestLogin,
      id: requestId,
    });
    setProcessingRequestId(null);

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось одобрить заявку",
        color: "red",
      });
      return;
    }

    notifications.show({
      message: `Заявка пользователя @${username} одобрена`,
      color: "green",
    });
    loadRequests();
    loadParticipants();
  };

  const handleRejectRequest = async (requestId: string, username: string) => {
    setProcessingRequestId(requestId);
    const [error] = await api.rejectContestJoinRequest({
      orgLogin,
      contestLogin,
      id: requestId,
    });
    setProcessingRequestId(null);

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось отклонить заявку",
        color: "red",
      });
      return;
    }

    notifications.show({
      message: `Заявка пользователя @${username} отклонена`,
      color: "blue",
    });
    loadRequests();
  };

  return (
    <>
      <Tabs value={activeTab} onChange={setActiveTab} mb="md">
        <Tabs.List>
          <Tabs.Tab
            value="users"
            leftSection={<IconUser size={16} />}
            rightSection={
              participants.length > 0 ? (
                <Badge size="xs" variant="light">
                  {participants.length}
                </Badge>
              ) : null
            }
          >
            Пользователи
          </Tabs.Tab>
          <Tabs.Tab value="teams" leftSection={<IconUsersGroup size={16} />}>
            Команды
          </Tabs.Tab>
          <Tabs.Tab
            value="requests"
            leftSection={<IconUserCheck size={16} />}
            rightSection={
              requests.length > 0 ? (
                <Badge size="xs" color="orange" variant="filled">
                  {requests.length}
                </Badge>
              ) : null
            }
          >
            Заявки
          </Tabs.Tab>
        </Tabs.List>

        <Tabs.Panel value="users" pt="md">
          <Stack gap="md">
            <Group gap="md">
              <Autocomplete
                placeholder="Поиск по имени пользователя..."
                value={searchQuery}
                onChange={(value) => {
                  setSearchQuery(value);
                  const selected = searchResults.find(
                    (u) => `${u.username} (${u.role})` === value
                  );
                  setSelectedUserId(selected ? selected.id : null);
                }}
                data={searchResults.map((u) => `${u.username} (${u.role})`)}
                style={{flex: 1}}
                leftSection={searching ? <Loader size="xs" /> : undefined}
              />
              <Button
                onClick={handleAddParticipant}
                loading={adding}
                disabled={!selectedUserId}
                leftSection={<IconPlus size={16} />}
              >
                Добавить
              </Button>
            </Group>

            {loading && (
              <Center py="xl">
                <Loader />
              </Center>
            )}
            {!loading && participants.length === 0 && (
              <Card withBorder p="xl">
                <Text c="dimmed" ta="center">
                  Участники не найдены
                </Text>
              </Card>
            )}
            {!loading && participants.length > 0 && (
              <Stack gap="md">
                <Table striped highlightOnHover withTableBorder withColumnBorders>
                  <Table.Thead>
                    <Table.Tr>
                      <Table.Th>Пользователь</Table.Th>
                      <Table.Th style={{textAlign: 'center'}}>Роль</Table.Th>
                      <Table.Th style={{width: 100}}>Действия</Table.Th>
                    </Table.Tr>
                  </Table.Thead>
                  <Table.Tbody>
                    {participants.map((user) => (
                      <Table.Tr key={user.user_id}>
                        <Table.Td>
                          <Text fw={500}>@{user.username}</Text>
                        </Table.Td>
                        <Table.Td style={{textAlign: 'center'}}>
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
                              <div style={{width: 28}} />
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
        </Tabs.Panel>

        <Tabs.Panel value="teams" pt="md">
          <ContestTeamsManagement orgLogin={orgLogin} contestLogin={contestLogin} contestId={contestId} orgId={orgId} />
        </Tabs.Panel>

        {/* Contest Join Requests Panel */}
        <Tabs.Panel value="requests" pt="md">
          <Stack gap="md">
            {loadingRequests && (
              <Center py="xl">
                <Loader />
              </Center>
            )}
            {!loadingRequests && requests.length === 0 && (
              <Card withBorder p="xl">
                <Text c="dimmed" ta="center">
                  Нет активных заявок на участие в контесте
                </Text>
              </Card>
            )}
            {!loadingRequests && requests.length > 0 && (
              <Table highlightOnHover withTableBorder>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Пользователь</Table.Th>
                    <Table.Th>Комментарий</Table.Th>
                    <Table.Th>Дата заявки</Table.Th>
                    <Table.Th style={{width: 180}}>Решение</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {requests.map((req) => (
                    <Table.Tr key={req.id}>
                      <Table.Td>
                        <Stack gap={2}>
                          <Text size="sm" fw={500}>
                            @{req.username}
                          </Text>
                          {req.email && (
                            <Text size="xs" c="dimmed">
                              {req.email}
                            </Text>
                          )}
                        </Stack>
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm" c={req.message ? undefined : "dimmed"}>
                          {req.message || "—"}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <Text size="sm" c="dimmed">
                          {new Date(req.created_at).toLocaleDateString("ru-RU")}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <Group gap="xs">
                          <Button
                            size="xs"
                            color="green"
                            variant="filled"
                            leftSection={<IconCheck size={14} />}
                            onClick={() => handleApproveRequest(req.id, req.username)}
                            loading={processingRequestId === req.id}
                          >
                            Одобрить
                          </Button>
                          <Button
                            size="xs"
                            color="red"
                            variant="light"
                            leftSection={<IconX size={14} />}
                            onClick={() => handleRejectRequest(req.id, req.username)}
                            loading={processingRequestId === req.id}
                          >
                            Отклонить
                          </Button>
                        </Group>
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            )}
          </Stack>
        </Tabs.Panel>
      </Tabs>

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
};
