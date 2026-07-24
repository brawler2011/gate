"use client";

import {
  Badge,
  Box,
  Center,
  Container,
  Group,
  Skeleton,
  Stack,
  Table,
  Text,
  Title,
} from "@mantine/core";
import type { SubmissionModel } from "@contracts/core/v1";
import { NextPagination } from '@/components/shared/Pagination';
import { LangString, StateColor, StateString, TimeBeautify } from "@/lib/lib";
import useSWR from "swr";
import Link from "next/link";
import classes from "./AdminPage.module.css";

type AdminSubmissionsContentProps = {
  page: number;
};

export function AdminSubmissionsContent({ page }: AdminSubmissionsContentProps) {
  const { data, error, isLoading } = useSWR(
    `/api/submissions?page=${page}&pageSize=10`,
    async (url) => {
      const res = await fetch(url);
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        throw new Error(errData.message || "Не удалось загрузить посылки");
      }
      return res.json();
    }
  );

  const submissions = data?.submissions || [];
  const pagination = data?.pagination || { total: 0, page: page };

  if (error) {
    return (
      <Container size="xl" py="md">
        <Center>
          <Stack align="center">
            <Title order={2}>Ошибка при загрузке посылок</Title>
            <Text c="dimmed">{error.message}</Text>
          </Stack>
        </Center>
      </Container>
    );
  }

  const totalPages = pagination.total || 1;

  return (
    <Container size="xl" py="md">
      <Stack gap="md">
        <Group justify="space-between" align="center">
          <Title order={3}>Посылки</Title>
        </Group>

        {isLoading ? (
          <Stack gap="sm">
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
          </Stack>
        ) : submissions.length === 0 ? (
          <Center py="xl">
            <Text c="dimmed">Посылки не найдены</Text>
          </Center>
        ) : (
          <>
            <Box className={classes.tableContainer}>
              <Table className={classes.table} verticalSpacing="xs">
                <Table.Thead className={classes.thead}>
                  <Table.Tr>
                    <Table.Th style={{ width: "15%" }}>ID</Table.Th>
                    <Table.Th style={{ width: "20%" }}>Задача</Table.Th>
                    <Table.Th style={{ width: "15%" }}>Контест</Table.Th>
                    <Table.Th style={{ width: "15%" }}>Отправитель</Table.Th>
                    <Table.Th style={{ width: "10%" }}>Язык</Table.Th>
                    <Table.Th style={{ width: "15%" }}>Вердикт</Table.Th>
                    <Table.Th style={{ width: "10%" }}>Когда</Table.Th>
                  </Table.Tr>
                </Table.Thead>
                <Table.Tbody className={classes.tbody}>
                  {submissions.map((submission: SubmissionModel) => (
                    <Table.Tr key={submission.id}>
                      <Table.Td>
                        <Link href={`/submissions/${submission.id}`} style={{ textDecoration: "none" }}>
                          <Text c="blue" fw={500}>
                            {submission.id.slice(0, 8)}...
                          </Text>
                        </Link>
                      </Table.Td>
                      <Table.Td>
                        <Link href={`/problems/${submission.problem_id}`} style={{ textDecoration: "none" }}>
                          <Text c="blue" lineClamp={1}>
                            {submission.problem_title || submission.problem_id.slice(0, 8)}
                          </Text>
                        </Link>
                      </Table.Td>
                      <Table.Td>
                        {submission.contest_id ? (
                          <Link href={`/contests/${submission.contest_id}`} style={{ textDecoration: "none" }}>
                            <Text c="blue" lineClamp={1}>
                              {submission.contest_title || submission.contest_id.slice(0, 8)}
                            </Text>
                          </Link>
                        ) : (
                          <Text c="dimmed">—</Text>
                        )}
                      </Table.Td>
                      <Table.Td>
                        <Link href={`/users/${submission.user_id}`} style={{ textDecoration: "none" }}>
                          <Text c="blue" lineClamp={1}>
                            {submission.username || submission.user_id.slice(0, 8)}
                          </Text>
                        </Link>
                      </Table.Td>
                      <Table.Td>
                        <Badge variant="light" color="gray" tt="none">
                          {LangString(submission.language)}
                        </Badge>
                      </Table.Td>
                      <Table.Td>
                        <Text c={StateColor(submission.state)} fw={500}>
                          {StateString(submission.state)}
                        </Text>
                      </Table.Td>
                      <Table.Td>
                        <Text className={classes.dateCell}>
                          {TimeBeautify(submission.created_at)}
                        </Text>
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
                  baseUrl="/admin/submissions"
                />
              </Stack>
            )}
          </>
        )}
      </Stack>
    </Container>
  );
}
