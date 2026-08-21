"use client";

import {
  Center,
  Container,
  Skeleton,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {useState} from "react";
import useSWR from "swr";

import {NextPagination} from '@/components/shared/Pagination';
import {StatusMessage} from '@/components/shared/StatusMessage';
import {api} from "@/lib/api";

import {AdminContestsSearchInput} from "./AdminContestsSearchInput";
import {AdminContestsTable} from "./AdminContestsTable";

import type {ReactNode} from "react";

type AdminContestsContentProps = {
  page: number;
  search?: string;
};

export const AdminContestsContent = ({page, search}: AdminContestsContentProps): ReactNode => {
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  const {data, error, isLoading, mutate} = useSWR(
    ["admin-contests", page, search],
    async () => {
      const [err, res] = await api.listAdminContests({page, pageSize: 10, search});
      if (err) {
        throw new Error(err.message);
      }
      return res;
    }
  );

  const contests = data?.contests || [];
  const pagination = data?.pagination || {total: 0, page: page};

  const handleDeleteContest = async (contest: {id: string; organization_login: string; login: string}) => {
    const [err] = await api.deleteContest({orgLogin: contest.organization_login, contestLogin: contest.login});
    
    if (err) {
      console.error("Error deleting contest:", err);
      notifications.show({
        title: "Ошибка",
        message: err.message || "Не удалось удалить контест",
        color: "red",
      });
      throw new Error(err.message);
    }

    setStatusMessage({
      type: "success",
      message: "Контест успешно удалён",
    });
    // Reload contests after deletion
    mutate();
  };

  if (error) {
    return (
      <Container size="xl" py="md">
        <Center>
          <Stack align="center">
            <Title order={2}>Ошибка при загрузке контестов</Title>
            <Text c="dimmed">Не удалось загрузить контесты</Text>
          </Stack>
        </Center>
      </Container>
    );
  }

  const totalPages = pagination.total || 1;
  const queryParams: Record<string, string | number | undefined> = {};
  if (search) {
    queryParams.search = search;
  }

  return (
    <Container size="xl" py="md">
      <Stack gap="md">
        <AdminContestsSearchInput />

        {isLoading && (
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
        )}
        {!isLoading && contests.length === 0 && (
          <Center py="xl">
            <Text c="dimmed">Контесты не найдены</Text>
          </Center>
        )}
        {!isLoading && contests.length > 0 && (
          <>
            <AdminContestsTable
              contests={contests}
              onDeleteContest={handleDeleteContest}
            />
            {totalPages > 1 && (
              <Stack align="center" gap="md">
                <NextPagination
                  pagination={{
                    page: page,
                    total: totalPages,
                  }}
                  baseUrl="/admin/contests"
                  queryParams={queryParams}
                />
              </Stack>
            )}
          </>
        )}
      </Stack>

      <StatusMessage
        type={statusMessage?.type || "success"}
        message={statusMessage?.message || ""}
        opened={!!statusMessage}
        onClose={() => setStatusMessage(null)}
      />
    </Container>
  );
};
