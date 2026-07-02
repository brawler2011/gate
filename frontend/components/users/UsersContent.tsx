"use client";

import { UsersRoleFilter } from '@/components/users/UsersRoleFilter';
import { UsersSearchInput } from '@/components/users/UsersSearchInput';
import { UsersTable } from '@/components/users/UsersTable';
import { listUsers } from "@/lib/actions";
import { Center, Container, Group, Skeleton, Stack, Text, Title } from "@mantine/core";
import type { UserModel } from "@contracts/core/v1";
import useSWR from "swr";

type UsersContentProps = {
  page: number;
  search?: string;
  role?: string;
};

export function UsersContent({ page, search, role }: UsersContentProps) {
  const { data, error, isLoading } = useSWR(
    ["admin", "users", page, search, role],
    async () => {
      const [err, res] = await listUsers(page, 10, search, role);
      if (err) throw err;
      return res;
    }
  );

  const users = data?.users || [];
  const pagination = data?.pagination || { total: 0, page: page };

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

        {isLoading ? (
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
