"use client";

import { Badge, Stack, Table, Text } from "@mantine/core";
import { useRouter } from "next/navigation";
import type {
  PaginationModel as PaginationType,
  UserModel,
} from "@contracts/core/v1";
import { NextPagination } from '@/components/shared/Pagination';
import { TruncatedWithCopy } from '@/components/shared/TruncatedWithCopy';
import { getRoleColor } from "@/lib/lib";

type Props = {
  users: UserModel[];
  pagination: PaginationType;
  page: number;
  search?: string;
  role?: string;
};

export function UsersTable({ users, pagination, page, search, role }: Props) {
  const router = useRouter();

  // Use page from URL props, not from API response state
  const currentPage = page;
  const totalPages = Number(pagination.total) || 1;

  if (users.length === 0) {
    return (
      <Text c="dimmed" ta="center" py="xl">
        {search || role
          ? "Пользователи по вашему запросу не найдены"
          : "Пользователи не найдены"}
      </Text>
    );
  }

  const queryParams: Record<string, string | number | undefined> = {};
  if (search) queryParams.search = search;
  if (role) queryParams.role = role;

  return (
    <>
      <Table striped highlightOnHover style={{ tableLayout: "fixed" }}>
        <Table.Thead>
          <Table.Tr>
            <Table.Th style={{ width: "30%" }}>Имя пользователя</Table.Th>
            <Table.Th style={{ width: "15%" }}>ID</Table.Th>
            <Table.Th style={{ width: "10%" }}>Роль</Table.Th>
            <Table.Th style={{ width: "10%" }}>Дата создания</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {users.map((user: UserModel) => (
            <Table.Tr
              key={user.id}
              onClick={(e) => {
                // Ignore clicks on buttons and interactive elements
                if ((e.target as HTMLElement).closest('button')) {
                  return;
                }
                router.push(`/users/${user.id}`);
              }}
              style={{ cursor: "pointer" }}
            >
              <Table.Td style={{ maxWidth: 0, overflow: "hidden" }}>{user.username}</Table.Td>
              <Table.Td style={{ maxWidth: 0, overflow: "hidden" }}>
                <TruncatedWithCopy value={user.id} />
              </Table.Td>
              <Table.Td style={{ maxWidth: 0, overflow: "hidden" }}>
                <Badge color={getRoleColor(user.role)}>{user.role}</Badge>
              </Table.Td>
              <Table.Td style={{ maxWidth: 0, overflow: "hidden" }}>
                {new Date(user.createdAt).toLocaleDateString("ru-RU")}
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>

      {totalPages > 1 && (
        <Stack align="center" gap="md">
          <NextPagination
            pagination={{
              page: currentPage,
              total: totalPages,
            }}
            baseUrl="/admin"
            queryParams={queryParams}
          />
        </Stack>
      )}
    </>
  );
}
