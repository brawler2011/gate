"use client";

import { deleteContest, listAdminContests } from "@/lib/actions";
import {
  Center,
  Container,
  Skeleton,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { useEffect, useState } from "react";
import type { ContestModel, PaginationModel } from "@contracts/core/v1";
import { NextPagination } from "../Pagination";
import { StatusMessage } from "../StatusMessage";
import { AdminContestsSearchInput } from "./AdminContestsSearchInput";
import { AdminContestsTable } from "./AdminContestsTable";

type AdminContestsContentProps = {
  page: number;
  search?: string;
};

export function AdminContestsContent({ page, search }: AdminContestsContentProps) {
  const [contests, setContests] = useState<ContestModel[]>([]);
  const [pagination, setPagination] = useState<PaginationModel>({
    total: 0,
    page: page,
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  const loadContests = async () => {
    setLoading(true);
    setError(false);

    const [err, data] = await listAdminContests(page, 10, search);

    if (err || !data) {
      console.error("Error fetching contests:", err);
      setError(true);
      setLoading(false);
      return;
    }

    setContests(data.contests || []);
    setPagination(data.pagination || { total: 0, page: 1 });
    setLoading(false);
  };

  useEffect(() => {
    setPagination((prev) => ({ ...prev, page }));
    loadContests();
  }, [page, search]);

  const handleDeleteContest = async (contestId: string) => {
    const [err] = await deleteContest(contestId);
    
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
    await loadContests();
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
  const queryParams: Record<string, string | number | undefined> = { view: "contests" };
  if (search) queryParams.search = search;

  return (
    <Container size="xl" py="md">
      <Stack gap="md">
        <Title order={3}>Контесты</Title>

        <AdminContestsSearchInput />

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
        ) : contests.length === 0 ? (
          <Center py="xl">
            <Text c="dimmed">Контесты не найдены</Text>
          </Center>
        ) : (
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
                  baseUrl="/admin"
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
}
