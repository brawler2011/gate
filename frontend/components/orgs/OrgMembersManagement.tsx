"use client";

import { StatusMessage } from "@/components/shared/StatusMessage";
import {
  addOrganizationMember,
  listOrganizationMembers,
  removeOrganizationMember,
  searchUsers,
} from "@/lib/actions";
import type { OrganizationMemberModel } from "@contracts/core/v1";
import {
  ActionIcon,
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
import { useDebouncedValue } from "@mantine/hooks";
import { notifications } from "@mantine/notifications";
import { IconPlus, IconTrash, IconUsers } from "@tabler/icons-react";
import { useCallback, useEffect, useState } from "react";

const ROLE_OPTIONS = [
  { label: "Владелец", value: "owner", color: "red" },
  { label: "Администратор", value: "admin", color: "orange" },
  { label: "Участник", value: "member", color: "blue" },
];

function getRoleDisplay(role: string) {
  return (
    ROLE_OPTIONS.find((r) => r.value === role) ?? { label: role, color: "gray" }
  );
}

type Props = { orgId: string };

export function OrgMembersManagement({ orgId }: Props) {
  const [members, setMembers] = useState<OrganizationMemberModel[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedQuery] = useDebouncedValue(searchQuery, 300);
  const [searchResults, setSearchResults] = useState<
    { value: string; label: string }[]
  >([]);
  const [searching, setSearching] = useState(false);
  const [selectedUserId, setSelectedUserId] = useState<string | null>(null);
  const [selectedRole, setSelectedRole] = useState<string>("member");
  const [adding, setAdding] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [status, setStatus] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  const loadMembers = useCallback(async () => {
    setLoading(true);
    const [, data] = await listOrganizationMembers(orgId, 1, 100);
    setLoading(false);
    if (data) setMembers(data.members);
  }, [orgId]);

  useEffect(() => {
    loadMembers();
  }, [loadMembers]);

  useEffect(() => {
    if (!debouncedQuery || debouncedQuery.length < 2) {
      setSearchResults([]);
      return;
    }
    setSearching(true);
    searchUsers(debouncedQuery).then(([, data]) => {
      setSearching(false);
      setSearchResults(
        (data?.users ?? []).map((u) => ({ value: u.id, label: u.username })),
      );
    });
  }, [debouncedQuery]);

  const handleAdd = async () => {
    if (!selectedUserId) return;
    setAdding(true);
    const [error] = await addOrganizationMember(
      orgId,
      selectedUserId,
      selectedRole as "owner" | "admin" | "member",
    );
    setAdding(false);
    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message,
        color: "red",
      });
      setStatus({ type: "error", message: error.message });
      return;
    }
    setStatus({ type: "success", message: "Участник добавлен" });
    setSearchQuery("");
    setSelectedUserId(null);
    await loadMembers();
  };

  const handleRemove = async (userId: string) => {
    setDeletingId(userId);
    const [error] = await removeOrganizationMember(orgId, userId);
    setDeletingId(null);
    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message,
        color: "red",
      });
      setStatus({ type: "error", message: error.message });
      return;
    }
    setStatus({ type: "success", message: "Участник удалён" });
    await loadMembers();
  };

  return (
    <>
      <Stack gap="md">
        <Card withBorder padding="lg">
          <Group gap="md" align="flex-end">
            <Select
              size="md"
              placeholder="Поиск пользователя..."
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
              style={{ flex: 1 }}
            />
            <Select
              size="md"
              data={ROLE_OPTIONS}
              value={selectedRole}
              onChange={(v) => setSelectedRole(v ?? "member")}
              w={180}
            />
            <Button
              size="md"
              onClick={handleAdd}
              loading={adding}
              disabled={!selectedUserId}
              leftSection={<IconPlus size={16} />}
            >
              Добавить
            </Button>
          </Group>
        </Card>

        {loading ? (
          <Center py="xl">
            <Loader />
          </Center>
        ) : members.length === 0 ? (
          <Center py="xl">
            <Stack align="center" gap="sm">
              <IconUsers size={32} color="var(--mantine-color-dimmed)" />
              <Text size="sm" c="dimmed">
                Нет участников
              </Text>
            </Stack>
          </Center>
        ) : (
          <Table highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                <Table.Th>Пользователь</Table.Th>
                <Table.Th>Роль</Table.Th>
                <Table.Th>Добавлен</Table.Th>
                <Table.Th style={{ width: 60 }}>Действия</Table.Th>
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {members.map((m) => {
                const role = getRoleDisplay(m.role);
                return (
                  <Table.Tr key={m.user_id}>
                    <Table.Td>
                      <Text size="sm" fw={500}>
                        {m.username}
                      </Text>
                    </Table.Td>
                    <Table.Td>
                      <Badge
                        color={role.color}
                        variant="filled"
                        size="md"
                        tt="none"
                      >
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
                        <ActionIcon
                          color="red"
                          variant="subtle"
                          onClick={() => handleRemove(m.user_id)}
                          loading={deletingId === m.user_id}
                        >
                          <IconTrash size={16} />
                        </ActionIcon>
                      )}
                    </Table.Td>
                  </Table.Tr>
                );
              })}
            </Table.Tbody>
          </Table>
        )}
      </Stack>
      <StatusMessage
        type={status?.type ?? "success"}
        message={status?.message ?? ""}
        opened={!!status}
        onClose={() => setStatus(null)}
      />
    </>
  );
}
