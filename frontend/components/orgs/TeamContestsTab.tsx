import {Badge, Card, Center, Skeleton, Stack, Table, Text} from "@mantine/core";
import {IconTrophy} from "@tabler/icons-react";
import {useQuery} from "@tanstack/react-query";
import {Link} from "react-router-dom";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";

import type {ContestModel} from "@/contracts/core/v1/models";
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
  const {data, isLoading} = useQuery({queryKey: ["team-contests", teamId], queryFn: async () => ({
    result: await api.listTeamContests(teamId, {}),
    loadedAt: Date.now(),
  })});

  if (isLoading || !data) {
    return (
      <Stack role="status" aria-label="Загрузка контестов команды">
        <Skeleton height={40} />
        <Skeleton height={40} />
        <Skeleton height={40} />
      </Stack>
    );
  }

  const [error, response] = data.result;
  if (error) {
    return <ErrorDisplay error={error} />;
  }
  const contests = response.contests;

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
                {getContestStatus(c, data.loadedAt)}
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
    </Card>
  );
};
