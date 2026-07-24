"use client";

import { deleteProblem } from "@/lib/actions";
import {
  ActionIcon,
  Badge,
  Box,
  Center,
  Container,
  Group,
  Skeleton,
  Stack,
  Table,
  Text,
  TextInput,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconSearch, IconTrash } from "@tabler/icons-react";
import { useEffect, useRef, useState } from "react";
import type { ProblemsListItemModel } from "@contracts/core/v1";
import { NextPagination } from '@/components/shared/Pagination';
import { TruncatedWithCopy } from '@/components/shared/TruncatedWithCopy';
import { DeleteProblemModal } from "./DeleteProblemModal";
import useSWR from "swr";
import { useRouter, useSearchParams } from "next/navigation";
import classes from "./AdminPage.module.css";

type AdminProblemsContentProps = {
  page: number;
  search?: string;
};

export function AdminProblemsContent({ page, search }: AdminProblemsContentProps) {
  const router = useRouter();
  const searchParams = useSearchParams();

  // Search input state
  const [searchInput, setSearchInput] = useState(search || "");
  const searchTimeoutRef = useRef<NodeJS.Timeout>();
  const isFirstRender = useRef(true);

  // Delete modal state
  const [deleteModalOpened, setDeleteModalOpened] = useState(false);
  const [problemToDelete, setProblemToDelete] = useState<ProblemsListItemModel | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  // Sync state with URL when searchParams change externally
  useEffect(() => {
    setSearchInput(search || "");
  }, [search]);

  // Handle search input change with debounce
  useEffect(() => {
    if (isFirstRender.current) {
      isFirstRender.current = false;
      return;
    }

    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    searchTimeoutRef.current = setTimeout(() => {
      const params = new URLSearchParams(searchParams);
      params.delete("page"); // Reset to first page
      if (searchInput) {
        params.set("search", searchInput);
      } else {
        params.delete("search");
      }
      router.push(`/admin/problems?${params.toString()}`);
    }, 300);

    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current);
      }
    };
  }, [searchInput, router, searchParams]);

  // Fetch problems
  const { data, error, isLoading, mutate } = useSWR(
    `/api/problems?page=${page}&pageSize=10${search ? `&search=${encodeURIComponent(search)}` : ""}`,
    async (url) => {
      const res = await fetch(url);
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        throw new Error(errData.message || "Не удалось загрузить задачи");
      }
      return res.json();
    }
  );

  const problems = data?.problems || [];
  const pagination = data?.pagination || { total: 0, page: page };

  const handleDeleteClick = (e: React.MouseEvent, problem: ProblemsListItemModel) => {
    e.stopPropagation();
    setProblemToDelete(problem);
    setDeleteModalOpened(true);
  };

  const handleDeleteConfirm = async () => {
    if (!problemToDelete) return;

    setDeletingId(problemToDelete.id);
    try {
      const [err] = await deleteProblem(problemToDelete.id);
      if (err) {
        notifications.show({
          title: "Ошибка",
          message: err.message || "Не удалось удалить задачу",
          color: "red",
        });
        throw new Error(err.message);
      }

      notifications.show({
        title: "Успех",
        message: "Задача успешно удалена",
        color: "green",
      });
      mutate();
    } finally {
      setDeletingId(null);
      setProblemToDelete(null);
    }
  };

  if (error) {
    return (
      <Container size="xl" py="md">
        <Center>
          <Stack align="center">
            <Title order={2}>Ошибка при загрузке задач</Title>
            <Text c="dimmed">{error.message}</Text>
          </Stack>
        </Center>
      </Container>
    );
  }

  const totalPages = pagination.total || 1;
  const queryParams: Record<string, string | number | undefined> = {};
  if (search) queryParams.search = search;

  return (
    <Container size="xl" py="md">
      <Stack gap="md">
        <Group justify="space-between" align="center">
          <Title order={3}>Задачи</Title>
        </Group>

        <TextInput
          placeholder="Поиск задач..."
          leftSection={<IconSearch size={16} />}
          value={searchInput}
          onChange={(e) => setSearchInput(e.currentTarget.value)}
          style={{ maxWidth: 400 }}
        />

        {isLoading ? (
          <Stack gap="sm">
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
          </Stack>
        ) : problems.length === 0 ? (
          <Center py="xl">
            <Text c="dimmed">Задачи не найдены</Text>
          </Center>
        ) : (
          <>
            <Box className={classes.tableContainer}>
              <Table className={classes.table} verticalSpacing="xs">
                <Table.Thead className={classes.thead}>
                  <Table.Tr>
                    <Table.Th style={{ width: "30%" }}>Название</Table.Th>
                    <Table.Th style={{ width: "20%" }}>ID</Table.Th>
                    <Table.Th style={{ width: "10%" }}>Время</Table.Th>
                    <Table.Th style={{ width: "10%" }}>Память</Table.Th>
                    <Table.Th style={{ width: "10%" }}>Шаблон</Table.Th>
                    <Table.Th style={{ width: "10%" }}>Создана</Table.Th>
                    <Table.Th style={{ width: "10%" }}>Действия</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody className={classes.tbody}>
                  {problems.map((problem: ProblemsListItemModel) => (
                    <Table.Tr
                      key={problem.id}
                      onClick={() => router.push(`/problems/${problem.id}`)}
                    >
                      <Table.Td>
                        <Text className={classes.titleCell} lineClamp={1}>
                          {problem.title}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <TruncatedWithCopy value={problem.id} />
                      </Table.Td>
                      <Table.Td>
                        <Text className={classes.metaCell}>{problem.time_limit} мс</Text>
                      </Table.Td>
                      <Table.Td>
                        <Text className={classes.metaCell}>{problem.memory_limit} МБ</Text>
                      </Table.Td>
                      <Table.Td>
                        {problem.is_template ? (
                          <Badge color="blue" variant="light">Да</Badge>
                        ) : (
                          <Badge color="gray" variant="light">Нет</Badge>
                        )}
                      </Table.Td>
                      <Table.Td>
                        <Text className={classes.dateCell}>
                          {new Date(problem.created_at).toLocaleDateString("ru-RU")}
                        </Text>
                      </Table.Td>
                      <Table.Td className={classes.actionsCell}>
                        <Group gap="xs" wrap="nowrap">
                          <ActionIcon
                            color="red"
                            variant="subtle"
                            onClick={(e) => handleDeleteClick(e, problem)}
                            loading={deletingId === problem.id}
                          >
                            <IconTrash size={16} />
                          </ActionIcon>
                        </Group>
                      </Table.Td>
                    </Table.Tr>
                  ))}
                </Table.Tbody>
              </Table>
            </Box>

            {totalPages > 1 && (
              <Stack align="center" gap="md">
                <NextPagination
                  pagination={{
                    page: page,
                    total: totalPages,
                  }}
                  baseUrl="/admin/problems"
                  queryParams={queryParams}
                />
              </Stack>
            )}
          </>
        )}
      </Stack>

      {problemToDelete && (
        <DeleteProblemModal
          opened={deleteModalOpened}
          onClose={() => {
            setDeleteModalOpened(false);
            setProblemToDelete(null);
          }}
          problem={{
            id: problemToDelete.id,
            title: problemToDelete.title,
          }}
          onSubmit={handleDeleteConfirm}
        />
      )}
    </Container>
  );
}
