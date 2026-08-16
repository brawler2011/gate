"use client";

import {ActionIcon, Badge, Group, Select, Stack, Table, Text} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconFileCode} from "@tabler/icons-react";
import {useRouter} from "next/navigation";
import {useState} from "react";

import {NextPagination} from '@/components/shared/Pagination';
import {TruncatedWithCopy} from '@/components/shared/TruncatedWithCopy';
import {api} from "@/lib/api";
import {getRoleColor} from "@/lib/lib";

import type {
  PaginationModel as PaginationType,
  UserModel,
  UpdateUserRequestModel,
} from "@/contracts/core/v1";
import type {ReactNode} from "react";

type Props = {
  users: UserModel[];
  pagination: PaginationType;
  page: number;
  search?: string;
  role?: string;
  onRefresh?: () => void;
};

export const UsersTable = ({users, pagination, page, search, role, onRefresh}: Props): ReactNode => {
  const router = useRouter();
  const [updatingId, setUpdatingId] = useState<string | null>(null);

  const currentPage = page;
  const totalPages = Number(pagination.total) || 1;

  const handleRoleChange = async (userId: string, newRole: string) => {
    setUpdatingId(userId);
    try {
      const [err] = await api.updateUser({
        id: userId,
        requestBody: {
          role: newRole as UpdateUserRequestModel.role,
        },
      });

      if (err) {
        notifications.show({
          title: "Ошибка",
          message: err.message || "Не удалось обновить роль пользователя",
          color: "red",
        });
        return;
      }

      notifications.show({
        title: "Успех",
        message: "Роль пользователя успешно обновлена",
        color: "green",
      });

      if (onRefresh) {
        onRefresh();
      } else {
        router.refresh();
      }
    } finally {
      setUpdatingId(null);
    }
  };

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
  if (search) {
    queryParams.search = search;
  }
  if (role) {
    queryParams.role = role;
  }

  return (
    <>
      <Table striped highlightOnHover style={{tableLayout: "fixed"}}>
        <Table.Thead>
          <Table.Tr>
            <Table.Th style={{width: "25%"}}>Имя пользователя</Table.Th>
            <Table.Th style={{width: "25%"}}>ID</Table.Th>
            <Table.Th style={{width: "20%"}}>Роль</Table.Th>
            <Table.Th style={{width: "15%"}}>Дата создания</Table.Th>
            <Table.Th style={{width: "15%"}}>Действия</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {users.map((user: UserModel) => (
            <Table.Tr
              key={user.id}
              onClick={(e) => {
                const target = e.target as HTMLElement;
                if (target.closest('button') || target.closest('.mantine-Select-root') || target.closest('.mantine-ActionIcon-root')) {
                  return;
                }
                router.push(`/users/${user.id}`);
              }}
              style={{cursor: "pointer"}}
            >
              <Table.Td style={{maxWidth: 0, overflow: "hidden"}}>{user.username}</Table.Td>
              <Table.Td style={{maxWidth: 0, overflow: "hidden"}}>
                <TruncatedWithCopy value={user.id} />
              </Table.Td>
              <Table.Td style={{maxWidth: 0, overflow: "hidden"}} onClick={(e) => e.stopPropagation()}>
                <Select
                  size="xs"
                  value={user.role}
                  disabled={updatingId === user.id}
                  data={[
                    {value: "user", label: "Пользователь"},
                    {value: "admin", label: "Администратор"},
                  ]}
                  onChange={(val) => {
                    if (val && val !== user.role) {
                      handleRoleChange(user.id, val);
                    }
                  }}
                  style={{maxWidth: 150}}
                />
              </Table.Td>
              <Table.Td style={{maxWidth: 0, overflow: "hidden"}}>
                {new Date(user.createdAt).toLocaleDateString("ru-RU")}
              </Table.Td>
              <Table.Td style={{maxWidth: 0, overflow: "hidden"}} onClick={(e) => e.stopPropagation()}>
                <Group gap="xs">
                  <ActionIcon
                    variant="subtle"
                    color="blue"
                    title="Посмотреть посылки пользователя"
                    onClick={() => router.push(`/admin/submissions?userId=${user.id}`)}
                  >
                    <IconFileCode size={18} />
                  </ActionIcon>
                </Group>
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
            baseUrl="/admin/users"
            queryParams={queryParams}
          />
        </Stack>
      )}
    </>
  );
};

