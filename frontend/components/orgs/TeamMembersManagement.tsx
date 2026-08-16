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
  Select,
  Stack,
  Table,
  Text,
} from "@mantine/core";
import {useDebouncedValue} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {IconEdit, IconPlus, IconTrash, IconUsers} from "@tabler/icons-react";
import {useCallback, useEffect, useState} from "react";

import {ChangeRoleModal} from "@/components/contests/ChangeRoleModal";
import {StatusMessage} from "@/components/shared/StatusMessage";
import {api} from "@/lib/api";

import type {TeamMemberModel} from "@/contracts/core/v1";
import type {ReactNode} from "react";

type Props = { teamId: string };

const TEAM_ROLE_OPTIONS = [
  {label: "Участник (member)", value: "member", color: "gray"},
  {label: "Организатор (maintainer)", value: "maintainer", color: "blue"},
];

const getRoleDisplay = (role: string) => {
  const found = TEAM_ROLE_OPTIONS.find((r) => r.value === role);
  return found || {label: role || "member", color: "gray"};
};

export const TeamMembersManagement = ({teamId}: Props): ReactNode => {
  const [members, setMembers] = useState<TeamMemberModel[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedQuery] = useDebouncedValue(searchQuery, 300);
  const [searchResults, setSearchResults] = useState<{ value: string; label: string }[]>([]);
  const [searching, setSearching] = useState(false);
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);
  const [selectedRole, setSelectedRole] = useState<string>("member");
  const [adding, setAdding] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [status, setStatus] = useState<{ type: "success" | "error"; message: string } | null>(null);

  const [editingMember, setEditingMember] = useState<{
    username: string;
    userId: string;
    currentRole: string;
  } | null>(null);
  const [modalOpened, setModalOpened] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    const [, data] = await api.listTeamMembers({id: teamId, page: 1, pageSize: 100});
    setLoading(false);
    if (data) {
      setMembers(data.members);
    }
  }, [teamId]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (!debouncedQuery || debouncedQuery.length < 2) {
      setSearchResults([]);
      return;
    }
    setSearching(true);
    api.listUsers({page: 1, pageSize: 10, search: debouncedQuery}).then(([, data]) => {
      setSearching(false);
      setSearchResults((data?.users ?? []).map((u) => ({value: u.id, label: u.username})));
    });
  }, [debouncedQuery]);

  const handleAdd = async () => {
    if (!selectedUserId) {
      return;
    }
    setAdding(true);
    const [error] = await api.addTeamMember({id: teamId, userId: selectedUserId, role: selectedRole});
    setAdding(false);
    if (error) {
      notifications.show({title: "Ошибка", message: error.message, color: "red"});
      setStatus({type: "error", message: error.message});
      return;
    }
    setStatus({type: "success", message: "Участник добавлен"});
    setSearchQuery("");
    setSelectedUserId(null);
    await load();
  };

  const handleRemove = async (userId: string) => {
    setDeletingId(userId);
    const [error] = await api.removeTeamMember({id: teamId, userId});
    setDeletingId(null);
    if (error) {
      notifications.show({title: "Ошибка", message: error.message, color: "red"});
      setStatus({type: "error", message: error.message});
      return;
    }
    setStatus({type: "success", message: "Участник удалён"});
    await load();
  };

  const handleEditRole = (member: TeamMemberModel) => {
    setEditingMember({
      username: member.username,
      userId: member.user_id,
      currentRole: member.role || "member",
    });
    setModalOpened(true);
  };

  const handleChangeRole = async (newRole: string) => {
    if (!editingMember) {
      return;
    }

    const [error] = await api.updateTeamMemberRole({
      id: teamId,
      userId: editingMember.userId,
      role: newRole,
    });

    setModalOpened(false);

    if (error) {
      notifications.show({title: "Ошибка", message: error.message, color: "red"});
      setStatus({type: "error", message: error.message});
      return;
    }

    setStatus({type: "success", message: "Роль обновлена"});
    await load();
  };

  return (
    <>
      <Stack gap="md">
        <Card withBorder padding="md">
          <Group gap="sm">
            <Autocomplete
              placeholder="Поиск пользователя..."
              value={searchQuery}
              onChange={(v) => {
                setSearchQuery(v);
                setSelectedUserId(null);
              }}
              onOptionSubmit={(v) => {
                setSelectedUserId(v);
                setSearchQuery(searchResults.find((r) => r.value === v)?.label ?? v);
              }}
              data={searchResults}
              rightSection={searching ? <Loader size="xs" /> : null}
              style={{flex: 1}}
            />
            <Select
              value={selectedRole}
              onChange={(v) => setSelectedRole(v || "member")}
              data={[
                {label: "Участник", value: "member"},
                {label: "Организатор", value: "maintainer"},
              ]}
              style={{width: 150}}
            />
            <Button
              onClick={handleAdd}
              loading={adding}
              disabled={!selectedUserId}
              leftSection={<IconPlus size={16} />}
            >
              Добавить
            </Button>
          </Group>
        </Card>

        {loading && (
          <Center py="xl">
            <Loader />
          </Center>
        )}
        {!loading && members.length === 0 && (
          <Center py="xl">
            <Stack align="center" gap="sm">
              <IconUsers size={32} color="var(--mantine-color-dimmed)" />
              <Text size="sm" c="dimmed">
                Нет участников
              </Text>
            </Stack>
          </Center>
        )}
        {!loading && members.length > 0 && (
          <Table highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Пользователь</Table.Th>
                <Table.Th style={{textAlign: "center"}}>Роль</Table.Th>
                <Table.Th>Добавлен</Table.Th>
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
                    <Text size="sm" c="dimmed">
                      {new Date(m.created_at).toLocaleDateString("ru-RU")}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Group gap="xs" wrap="nowrap">
                      <ActionIcon
                        color="blue"
                        variant="subtle"
                        onClick={() => handleEditRole(m)}
                      >
                        <IconEdit size={16} />
                      </ActionIcon>
                      <ActionIcon
                        color="red"
                        variant="subtle"
                        onClick={() => handleRemove(m.user_id)}
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
        type={status?.type ?? "success"}
        message={status?.message ?? ""}
        opened={!!status}
        onClose={() => setStatus(null)}
      />
    </>
  );
};
