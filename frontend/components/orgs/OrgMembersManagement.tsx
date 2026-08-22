"use client";

import {
  ActionIcon,
  Badge,
  Button,
  Center,
  Group,
  Loader,
  Modal,
  Select,
  Stack,
  Table,
  Tabs,
  Text,
  Tooltip,
} from "@mantine/core";
import {useDebouncedValue, useDisclosure} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {
  IconCheck,
  IconMail,
  IconSend,
  IconTrash,
  IconUserCheck,
  IconUserPlus,
  IconUsers,
  IconX,
} from "@tabler/icons-react";
import {useCallback, useEffect, useState, type ReactNode} from "react";

import {BatchCreateUsersModal} from "@/components/orgs/BatchCreateUsersModal";
import {StatusMessage} from "@/components/shared/StatusMessage";
import {api} from "@/lib/api";

import type {
  ApproveOrganizationJoinRequestModel,
  InviteOrganizationMemberRequestModel,
  OrganizationInvitationModel,
  OrganizationJoinRequestModel,
  OrganizationMemberModel,
} from "@/contracts/core/v1";

const ROLE_OPTIONS = [
  {label: "Владелец", value: "owner", color: "red"},
  {label: "Администратор", value: "admin", color: "orange"},
  {label: "Участник", value: "member", color: "blue"},
];

const getRoleDisplay = (role: string) => {
  return (
    ROLE_OPTIONS.find((r) => r.value === role) ?? {label: role, color: "gray"}
  );
};

type Props = { orgLogin: string };

export const OrgMembersManagement = ({orgLogin}: Props): ReactNode => {
  const [activeTab, setActiveTab] = useState<string | null>("members");

  // Members state
  const [members, setMembers] = useState<OrganizationMemberModel[]>([]);
  const [loadingMembers, setLoadingMembers] = useState(true);
  const [deletingMemberId, setDeletingMemberId] = useState<string | null>(null);

  // Invitations state
  const [invitations, setInvitations] = useState<OrganizationInvitationModel[]>([]);
  const [loadingInvitations, setLoadingInvitations] = useState(false);
  const [cancelingInvitationId, setCancelingInvitationId] = useState<string | null>(null);

  // Requests state
  const [requests, setRequests] = useState<OrganizationJoinRequestModel[]>([]);
  const [loadingRequests, setLoadingRequests] = useState(false);
  const [processingRequestId, setProcessingRequestId] = useState<string | null>(null);
  const [approveModalOpened, {open: openApproveModal, close: closeApproveModal}] = useDisclosure(false);
  const [selectedRequest, setSelectedRequest] = useState<OrganizationJoinRequestModel | null>(null);
  const [approveRole, setApproveRole] = useState<string>("member");

  // Invite modal / user search state
  const [inviteModalOpened, {open: openInviteModal, close: closeInviteModal}] = useDisclosure(false);
  const [batchModalOpened, setBatchModalOpened] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedQuery] = useDebouncedValue(searchQuery, 300);
  const [searchResults, setSearchResults] = useState<{value: string; label: string}[]>([]);
  const [searching, setSearching] = useState(false);
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);
  const [selectedRole, setSelectedRole] = useState<string>("member");
  const [inviting, setInviting] = useState(false);

  const [status, setStatus] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  const loadMembers = useCallback(async () => {
    setLoadingMembers(true);
    const [, data] = await api.listOrganizationMembers({login: orgLogin, page: 1, pageSize: 100});
    setLoadingMembers(false);
    if (data) {
      setMembers(data.members);
    }
  }, [orgLogin]);

  const loadInvitations = useCallback(async () => {
    setLoadingInvitations(true);
    const [, data] = await api.listOrganizationInvitations({login: orgLogin});
    setLoadingInvitations(false);
    if (data) {
      setInvitations(data.invitations);
    }
  }, [orgLogin]);

  const loadRequests = useCallback(async () => {
    setLoadingRequests(true);
    const [, data] = await api.listOrganizationJoinRequests({login: orgLogin});
    setLoadingRequests(false);
    if (data) {
      setRequests(data.requests);
    }
  }, [orgLogin]);

  useEffect(() => {
    loadMembers();
    loadInvitations();
    loadRequests();
  }, [loadMembers, loadInvitations, loadRequests]);

  useEffect(() => {
    if (!debouncedQuery || debouncedQuery.length < 2) {
      setSearchResults([]);
      return;
    }
    setSearching(true);
    api.listUsers({page: 1, pageSize: 10, search: debouncedQuery}).then(([, data]) => {
      setSearching(false);
      setSearchResults(
        (data?.users ?? []).map((u) => ({value: u.id, label: u.username})),
      );
    });
  }, [debouncedQuery]);

  const handleInvite = async () => {
    if (!selectedUserId) {
      return;
    }
    setInviting(true);
    const [error] = await api.inviteOrganizationMember({
      login: orgLogin,
      requestBody: {
        user_id: selectedUserId,
        role: selectedRole as InviteOrganizationMemberRequestModel.role,
      },
    });
    setInviting(false);
    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message,
        color: "red",
      });
      return;
    }
    notifications.show({
      title: "Успех",
      message: "Приглашение отправлено пользователю",
      color: "green",
    });
    closeInviteModal();
    setSearchQuery("");
    setSelectedUserId(null);
    loadInvitations();
  };

  const handleCancelInvitation = async (invitationId: string) => {
    setCancelingInvitationId(invitationId);
    const [error] = await api.cancelOrganizationInvitation({
      login: orgLogin,
      id: invitationId,
    });
    setCancelingInvitationId(null);
    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message,
        color: "red",
      });
      return;
    }
    notifications.show({
      message: "Приглашение отменено",
      color: "blue",
    });
    loadInvitations();
  };

  const handleRemoveMember = async (userId: string) => {
    setDeletingMemberId(userId);
    const [error] = await api.removeOrganizationMember({login: orgLogin, userId});
    setDeletingMemberId(null);
    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message,
        color: "red",
      });
      return;
    }
    notifications.show({
      message: "Участник удалён",
      color: "blue",
    });
    loadMembers();
  };

  const handleApproveRequest = async () => {
    if (!selectedRequest) {
      return;
    }
    setProcessingRequestId(selectedRequest.id);
    const [error] = await api.approveOrganizationJoinRequest({
      login: orgLogin,
      id: selectedRequest.id,
      requestBody: {
        role: approveRole as ApproveOrganizationJoinRequestModel.role,
      },
    });
    setProcessingRequestId(null);
    closeApproveModal();
    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message,
        color: "red",
      });
      return;
    }
    notifications.show({
      message: `Заявка пользователя @${selectedRequest.username} одобрена`,
      color: "green",
    });
    loadRequests();
    loadMembers();
  };

  const handleRejectRequest = async (requestId: string, username: string) => {
    setProcessingRequestId(requestId);
    const [error] = await api.rejectOrganizationJoinRequest({
      login: orgLogin,
      id: requestId,
    });
    setProcessingRequestId(null);
    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message,
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
      <Stack gap="md">
        <Tabs value={activeTab} onChange={setActiveTab}>
          <Group justify="space-between" align="center" mb="md" wrap="wrap">
            <Tabs.List>
              <Tabs.Tab
                value="members"
                leftSection={<IconUsers size={16} />}
                rightSection={
                  members.length > 0 ? (
                    <Badge size="xs" variant="light">
                      {members.length}
                    </Badge>
                  ) : null
                }
              >
                Участники
              </Tabs.Tab>
              <Tabs.Tab
                value="invitations"
                leftSection={<IconMail size={16} />}
                rightSection={
                  invitations.length > 0 ? (
                    <Badge size="xs" color="blue" variant="filled">
                      {invitations.length}
                    </Badge>
                  ) : null
                }
              >
                Приглашения
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

            <Group gap="sm">
              <Button
                variant="filled"
                color="blue"
                onClick={openInviteModal}
                leftSection={<IconSend size={16} />}
              >
                Пригласить
              </Button>
              <Button
                variant="light"
                onClick={() => setBatchModalOpened(true)}
                leftSection={<IconUserPlus size={16} />}
              >
                Сгенерировать участников
              </Button>
            </Group>
          </Group>

          {/* Members Tab */}
          <Tabs.Panel value="members">
            {loadingMembers && (
              <Center py="xl">
                <Loader />
              </Center>
            )}
            {!loadingMembers && members.length === 0 && (
              <Center py="xl">
                <Stack align="center" gap="sm">
                  <IconUsers size={32} color="var(--mantine-color-dimmed)" />
                  <Text size="sm" c="dimmed">
                    Нет участников
                  </Text>
                </Stack>
              </Center>
            )}
            {!loadingMembers && members.length > 0 && (
              <Table highlightOnHover>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Пользователь</Table.Th>
                    <Table.Th>Роль</Table.Th>
                    <Table.Th>Добавлен</Table.Th>
                    <Table.Th style={{width: 60}}>Действия</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {members.map((m) => {
                    const role = getRoleDisplay(m.role);
                    return (
                      <Table.Tr key={m.user_id}>
                        <Table.Td>
                          <Text size="sm" fw={500}>
                            @{m.username}
                          </Text>
                        </Table.Td>
                        <Table.Td>
                          <Badge color={role.color} variant="filled" size="md" tt="none">
                            {role.label}
                          </Badge>
                        </Table.Td>
                        <Table.Td>
                          <Text size="sm" c="dimmed">
                            {new Date(m.created_at).toLocaleDateString("ru-RU")}
                          </Text>
                        </Table.Td>
                        <Table.Td>
                          {m.role !== "owner" && (
                            <Tooltip label="Удалить из организации">
                              <ActionIcon
                                color="red"
                                variant="subtle"
                                onClick={() => handleRemoveMember(m.user_id)}
                                loading={deletingMemberId === m.user_id}
                              >
                                <IconTrash size={16} />
                              </ActionIcon>
                            </Tooltip>
                          )}
                        </Table.Td>
                      </Table.Tr>
                    );
                  })}
                </Table.Tbody>
              </Table>
            )}
          </Tabs.Panel>

          {/* Invitations Tab */}
          <Tabs.Panel value="invitations">
            {loadingInvitations && (
              <Center py="xl">
                <Loader />
              </Center>
            )}
            {!loadingInvitations && invitations.length === 0 && (
              <Center py="xl">
                <Stack align="center" gap="sm">
                  <IconMail size={32} color="var(--mantine-color-dimmed)" />
                  <Text size="sm" c="dimmed">
                    Нет активных приглашений
                  </Text>
                </Stack>
              </Center>
            )}
            {!loadingInvitations && invitations.length > 0 && (
              <Table highlightOnHover>
                <Table.Thead>
                  <Table.Tr>
                    <Table.Th>Пользователь</Table.Th>
                    <Table.Th>Роль</Table.Th>
                    <Table.Th>Пригласил</Table.Th>
                    <Table.Th>Дата</Table.Th>
                    <Table.Th style={{width: 80}}>Действия</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody>
                  {invitations.map((inv) => {
                    const role = getRoleDisplay(inv.role);
                    return (
                      <Table.Tr key={inv.id}>
                        <Table.Td>
                          <Stack gap={2}>
                            <Text size="sm" fw={500}>
                              @{inv.username}
                            </Text>
                            {inv.email && (
                              <Text size="xs" c="dimmed">
                                {inv.email}
                              </Text>
                            )}
                          </Stack>
                        </Table.Td>
                        <Table.Td>
                          <Badge color={role.color} variant="filled" size="md" tt="none">
                            {role.label}
                          </Badge>
                        </Table.Td>
                        <Table.Td>
                          <Text size="sm" c="dimmed">
                            @{inv.inviter_username}
                          </Text>
                        </Table.Td>
                        <Table.Td>
                          <Text size="sm" c="dimmed">
                            {new Date(inv.created_at).toLocaleDateString("ru-RU")}
                          </Text>
                        </Table.Td>
                        <Table.Td>
                          <Button
                            size="xs"
                            color="red"
                            variant="subtle"
                            onClick={() => handleCancelInvitation(inv.id)}
                            loading={cancelingInvitationId === inv.id}
                          >
                            Отменить
                          </Button>
                        </Table.Td>
                      </Table.Tr>
                    );
                  })}
                </Table.Tbody>
              </Table>
            )}
          </Tabs.Panel>

          {/* Join Requests Tab */}
          <Tabs.Panel value="requests">
            {loadingRequests && (
              <Center py="xl">
                <Loader />
              </Center>
            )}
            {!loadingRequests && requests.length === 0 && (
              <Center py="xl">
                <Stack align="center" gap="sm">
                  <IconUserCheck size={32} color="var(--mantine-color-dimmed)" />
                  <Text size="sm" c="dimmed">
                    Нет активных заявок на вступление
                  </Text>
                </Stack>
              </Center>
            )}
            {!loadingRequests && requests.length > 0 && (
              <Table highlightOnHover>
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
                            onClick={() => {
                              setSelectedRequest(req);
                              setApproveRole("member");
                              openApproveModal();
                            }}
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
          </Tabs.Panel>
        </Tabs>
      </Stack>

      {/* Invite Member Modal */}
      <Modal
        opened={inviteModalOpened}
        onClose={closeInviteModal}
        title="Пригласить участника в организацию"
        centered
      >
        <Stack gap="md">
          <Select
            label="Пользователь"
            placeholder="Введите имя пользователя..."
            searchable
            clearable
            value={selectedUserId}
            onChange={setSelectedUserId}
            data={searchResults}
            searchValue={searchQuery}
            onSearchChange={setSearchQuery}
            nothingFoundMessage={
              searchQuery.length < 2
                ? "Введите минимум 2 символа"
                : "Пользователь не найден"
            }
            rightSection={searching ? <Loader size="xs" /> : null}
          />
          <Select
            label="Роль в организации"
            data={ROLE_OPTIONS}
            value={selectedRole}
            onChange={(v) => setSelectedRole(v ?? "member")}
          />
          <Group justify="flex-end" mt="md">
            <Button variant="default" onClick={closeInviteModal}>
              Отмена
            </Button>
            <Button
              onClick={handleInvite}
              loading={inviting}
              disabled={!selectedUserId}
              leftSection={<IconSend size={16} />}
            >
              Отправить приглашение
            </Button>
          </Group>
        </Stack>
      </Modal>

      {/* Approve Request Modal */}
      <Modal
        opened={approveModalOpened}
        onClose={closeApproveModal}
        title={`Одобрить заявку @${selectedRequest?.username}`}
        centered
      >
        <Stack gap="md">
          <Text size="sm">
            Выберите роль, с которой пользователь вступит в организацию:
          </Text>
          <Select
            label="Роль"
            data={ROLE_OPTIONS}
            value={approveRole}
            onChange={(v) => setApproveRole(v ?? "member")}
          />
          <Group justify="flex-end" mt="md">
            <Button variant="default" onClick={closeApproveModal}>
              Отмена
            </Button>
            <Button
              color="green"
              onClick={handleApproveRequest}
              loading={!!processingRequestId}
              leftSection={<IconCheck size={16} />}
            >
              Подтвердить
            </Button>
          </Group>
        </Stack>
      </Modal>

      <BatchCreateUsersModal
        opened={batchModalOpened}
        onClose={() => setBatchModalOpened(false)}
        orgLogin={orgLogin}
        onSuccess={loadMembers}
      />
      <StatusMessage
        type={status?.type ?? "success"}
        message={status?.message ?? ""}
        opened={!!status}
        onClose={() => setStatus(null)}
      />
    </>
  );
};
