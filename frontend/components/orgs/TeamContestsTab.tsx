import {Pagination, Badge, Card, Center, Skeleton, Stack, Table, Text} from "@mantine/core";
import {IconTrophy} from "@tabler/icons-react";
import {Link} from "react-router-dom";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {usePagedQuery} from "@/hooks/usePagedQuery";
import {api, unwrap} from "@/lib/api";

import type {ContestModel} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

interface TeamContestsTabProps {
  teamId: string;
}

const getContestStatus = (contest: ContestModel, now: number): string => {
  if (contest.end_time && new Date(contest.end_time).getTime() <= now) {
    return "Завершён";
  }
  if (contest.start_time && new Date(contest.start_time).getTime() > now) {
    return "Предстоящий";
  }
  return contest.start_time ? "Идёт" : "Без расписания";
};

export const TeamContestsTab = ({teamId}: TeamContestsTabProps): ReactNode => {
  const {data, isPending: isLoading, error, page, setPage, dataUpdatedAt} = usePagedQuery(
    ["team-contests", teamId],
    (page, pageSize) => unwrap(api.GET("/teams/{id}/contests", {params: {path: {id: teamId}, query: {page, pageSize}}})),
  );
  if (error) {
    return <ErrorDisplay error={error} />;
  }

  if (isLoading || !data) {
    return (
      <Stack role="status" aria-label="Загрузка контестов команды">
        <Skeleton height={40} />
        <Skeleton height={40} />
        <Skeleton height={40} />
      </Stack>
    );
  }

  const contests = data.contests;

  if (contests.length === 0) {
    return (
      <Center py="xl">
        <Stack align="center" gap="sm">
          <IconTrophy size={32} color="var(--mantine-color-dimmed)" />
          <Text size="lg" c="dimmed">
            Нет привязанных контестов
          </Text>
          <Text size="sm" c="dimmed">
            Команда пока не имеет доступа ни к одному контесту
          </Text>
        </Stack>
      </Center>
    );
  }

  return (
    <Card withBorder padding="md">
      <Table highlightOnHover>
        <Table.Thead>
          <Table.Tr>
            <Table.Th>Название контеста</Table.Th>
            <Table.Th>Статус</Table.Th>
            <Table.Th style={{textAlign: "center"}}>Видимость</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {contests.map((c) => (
            <Table.Tr key={c.id}>
              <Table.Td>
                <Text
                  component={Link}
                  to={`/${c.organization_login}/${c.login || c.id}`}
                  size="sm"
                  fw={500}
                  c="blue"
                  style={{textDecoration: "none"}}
                >
                  {c.title}
                </Text>
              </Table.Td>
              <Table.Td>
                {getContestStatus(c, dataUpdatedAt)}
              </Table.Td>
              <Table.Td style={{textAlign: "center"}}>
                <Badge variant="light" color={c.visibility === "public" ? "green" : "gray"}>
                  {c.visibility === "public" ? "Публичный" : "Приватный"}
                </Badge>
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
      {data.pagination.total_pages > 1 && (
        <Pagination value={page} onChange={setPage} total={data.pagination.total_pages} />
      )}
    </Card>
  );
};
