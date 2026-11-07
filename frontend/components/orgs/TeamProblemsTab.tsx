import {Pagination, Badge, Card, Center, Loader, Stack, Table, Text} from "@mantine/core";
import {IconFileCode} from "@tabler/icons-react";
import {Link} from "react-router-dom";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {usePagedQuery} from "@/hooks/usePagedQuery";
import {api, unwrap} from "@/lib/api";

import type {ReactNode} from "react";

interface TeamProblemsTabProps {
  teamId: string;
}

export const TeamProblemsTab = ({teamId}: TeamProblemsTabProps): ReactNode => {

  const {data, isPending: loading, error, page, setPage} = usePagedQuery(
    ["team-problems", teamId],
    (page, pageSize) => unwrap(api.GET("/teams/{id}/problems", {params: {path: {id: teamId}, query: {page, pageSize}}})),
  );
  const problems = data?.problems ?? [];

  if (error) {
    return <ErrorDisplay error={error} />;
  }

  if (loading) {
    return (
      <Center py="xl">
        <Loader size="md" />
      </Center>
    );
  }

  if (problems.length === 0) {
    return (
      <Center py="xl">
        <Stack align="center" gap="sm">
          <IconFileCode size={32} color="var(--mantine-color-dimmed)" />
          <Text size="lg" c="dimmed">
            Нет привязанных задач
          </Text>
          <Text size="sm" c="dimmed">
            Команда пока не имеет доступа ни к одной задаче
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
            <Table.Th>Название задачи</Table.Th>
            <Table.Th style={{textAlign: "center"}}>Видимость</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {problems.map((p) => (
            <Table.Tr key={p.id}>
              <Table.Td>
                <Text
                  component={Link}
                  to={`/${p.organization_login}/problems/${p.id}`}
                  size="sm"
                  fw={500}
                  c="blue"
                  style={{textDecoration: "none"}}
                >
                  {p.title}
                </Text>
              </Table.Td>
              <Table.Td style={{textAlign: "center"}}>
                <Badge variant="light" color={p.visibility === "public" ? "green" : "gray"}>
                  {p.visibility === "public" ? "Публичная" : "Приватная"}
                </Badge>
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
      {data && data.pagination.total_pages > 1 && (
        <Pagination value={page} onChange={setPage} total={data.pagination.total_pages} />
      )}
    </Card>
  );
};
