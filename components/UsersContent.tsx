"use client";

import { useEffect, useState } from "react";
import { UsersRoleFilter } from "@/components/UsersRoleFilter";
import { UsersSearchInput } from "@/components/UsersSearchInput";
import { UsersTable } from "@/components/UsersTable";
import { listUsers } from "@/lib/actions";
import { Center, Container, Group, Skeleton, Stack, Text, Title } from "@mantine/core";
import type { UserModel, PaginationModel } from "../../contracts/core/v1";

type UsersContentProps = {
  page: number;
  search?: string;
  role?: string;
};

export function UsersContent({ page, search, role }: UsersContentProps) {
  const [users, setUsers] = useState<UserModel[]>([]);
  const [pagination, setPagination] = useState<PaginationModel>({
    total: 0,
    page: 1,
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);

  useEffect(() => {
    const fetchUsers = async () => {
      setLoading(true);
      setError(false);

      try {
        const data = await listUsers(page, 10, search, role);
        
        if (!data) {
          throw new Error("Failed to fetch users");
        }

        setUsers(data.users || []);
        setPagination(data.pagination || { total: 0, page: 1, pageSize: 10 });
      } catch (err) {
        console.error("Error fetching users:", err);
        setError(true);
      } finally {
        setLoading(false);
      }
    };

    fetchUsers();
  }, [page, search, role]);

  if (error) {
    return (
      <Container size="xl" py="xl">
        <Center>
          <Stack align="center">
            <Title order={2}>Ошибка при загрузке пользователей</Title>
            <Text c="dimmed">Не удалось загрузить пользователей</Text>
          </Stack>
        </Center>
      </Container>
    );
  }

  return (
    <Container size="xl" py="xl">
      <Stack gap="lg">

        <Group grow>
          <UsersSearchInput />
          <UsersRoleFilter />
        </Group>

        {loading ? (
          <Stack gap="sm">
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
          </Stack>
        ) : (
          <UsersTable
            users={users}
            pagination={pagination}
            page={page}
            search={search}
            role={role}
          />
        )}
      </Stack>
    </Container>
  );
}

