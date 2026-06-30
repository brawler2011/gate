"use client";

import type { OrganizationMemberModel } from "@contracts/core/v1";
import { Badge, Table, Text } from "@mantine/core";

const ROLE_COLORS: Record<string, string> = {
  owner: "red",
  admin: "orange",
  member: "gray",
};
const ROLE_LABELS: Record<string, string> = {
  owner: "Владелец",
  admin: "Администратор",
  member: "Участник",
};

type Props = { members: OrganizationMemberModel[] };

export function OrgMembersTab({ members }: Props) {
  if (members.length === 0) {
    return (
      <Text c="dimmed" py="xl" ta="center">
        Нет участников
      </Text>
    );
  }
  return (
    <Table verticalSpacing="sm">
      <Table.Thead>
        <Table.Tr>
          <Table.Th>Пользователь</Table.Th>
          <Table.Th>Роль</Table.Th>
          <Table.Th>Добавлен</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {members.map((m) => (
          <Table.Tr key={m.user_id}>
            <Table.Td>
              <Text size="sm">{m.username}</Text>
            </Table.Td>
            <Table.Td>
              <Badge
                color={ROLE_COLORS[m.role] ?? "gray"}
                variant="light"
                size="sm"
              >
                {ROLE_LABELS[m.role] ?? m.role}
              </Badge>
            </Table.Td>
            <Table.Td>
              <Text size="sm" c="dimmed">
                {new Date(m.created_at).toLocaleDateString("ru-RU")}
              </Text>
            </Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  );
}
