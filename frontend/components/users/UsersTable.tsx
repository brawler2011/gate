import {ActionIcon, Group, Select, Stack, Table, Text} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconFileCode, IconSettings} from "@tabler/icons-react";
import {useQueryClient} from "@tanstack/react-query";
import {useState} from "react";
import {useRevalidator, useNavigate} from "react-router-dom";

import {RoutePagination} from '@/components/shared/Pagination';
import {TruncatedWithCopy} from '@/components/shared/TruncatedWithCopy';
import {api} from "@/lib/api";

import type {PaginationModel as PaginationType, UserModel, UpdateUserRequestModel} from "@/contracts/core/v1/schema";
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
  const navigate = useNavigate();
  const revalidator = useRevalidator();
  const queryClient = useQueryClient();
  const [updatingId, setUpdatingId] = useState<string | null>(null);

  const currentPage = page;
  const totalPages = Number(pagination.total) || 1;

  const handleRoleChange = async (user: UserModel, newRole: string) => {
    setUpdatingId(user.id);
    try {
      const {error: err} = await api.PATCH("/users/{username}", {
        params: {path: {username: user.username}},
        body: {
          role: newRole as UpdateUserRequestModel["role"],
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
        void queryClient.invalidateQueries({queryKey: api.queryOptions("get", "/users").queryKey});
        void revalidator.revalidate();
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
                navigate(`/@${user.username}`);
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
                      handleRoleChange(user, val);
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
                    title="Редактировать пользователя"
                    onClick={() => navigate(`/admin/users/${user.username}`)}
                  >
                    <IconSettings size={18} />
                  </ActionIcon>
                  <ActionIcon
                    variant="subtle"
                    color="gray"
                    title="Посмотреть посылки пользователя"
                    onClick={() => navigate(`/admin/submissions?userId=${user.id}`)}
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
          <RoutePagination
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
