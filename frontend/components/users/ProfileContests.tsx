"use client";

import {Anchor, Badge, Center, Stack, Table, Tabs, Text} from "@mantine/core";
import {IconTrophy} from "@tabler/icons-react";
import Link from "next/link";

import {NextPagination} from "@/components/shared/Pagination";
import {TimeBeautify} from "@/lib/lib";

import type {ContestModel} from "@/contracts/core/v1";
import type {ReactNode} from "react";

type ProfileContestsProps = {
  username: string;
  contests: ContestModel[];
  contestsPagination?: { page: number; total: number };
  contestsPage: number;
};

export const ProfileContests = ({
  username,
  contests,
  contestsPagination,
  contestsPage,
}: ProfileContestsProps): ReactNode => {
  return (
    <Tabs defaultValue="contests">
      <Tabs.List>
        <Tabs.Tab value="contests" leftSection={<IconTrophy size={16} />}>
          Контесты
        </Tabs.Tab>
      </Tabs.List>

      <Tabs.Panel value="contests" pt="md">
        {contests.length > 0 ? (
          <>
            <Table striped highlightOnHover>
              <Table.Thead>
                <Table.Tr>
                  <Table.Th>Название</Table.Th>
                  <Table.Th>Видимость</Table.Th>
                  <Table.Th>Дата создания</Table.Th>
                </Table.Tr>
              </Table.Thead>
              <Table.Tbody>
                {contests.map((contest) => (
                  <Table.Tr key={contest.id}>
                    <Table.Td>
                      <Anchor component={Link} href={`/${contest.organization_login}/contests/${contest.id}`} size="sm">
                        {contest.title}
                      </Anchor>
                    </Table.Td>
                    <Table.Td>
                      <Badge
                        color={contest.visibility === "public" ? "green" : "gray"}
                        variant="light"
                        size="sm"
                      >
                        {contest.visibility}
                      </Badge>
                    </Table.Td>
                    <Table.Td>
                      <Text size="sm" c="dimmed">
                        {TimeBeautify(contest.created_at)}
                      </Text>
                    </Table.Td>
                  </Table.Tr>
                ))}
              </Table.Tbody>
            </Table>
            {contestsPagination && contestsPagination.total > 1 && (
              <Stack align="center" mt="md">
                <NextPagination
                  pagination={{page: contestsPage, total: contestsPagination.total}}
                  baseUrl={`/@${username}`}
                  queryParams={{contestsPage}}
                />
              </Stack>
            )}
          </>
        ) : (
          <Center py="xl">
            <Stack align="center" gap="sm">
              <IconTrophy size={40} style={{opacity: 0.3}} />
              <Text c="dimmed">Нет публичных контестов</Text>
            </Stack>
          </Center>
        )}
      </Tabs.Panel>
    </Tabs>
  );
};
