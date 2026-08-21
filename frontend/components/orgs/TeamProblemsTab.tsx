"use client";

import {Badge, Card, Center, Loader, Stack, Table, Text} from "@mantine/core";
import {IconFileCode} from "@tabler/icons-react";
import Link from "next/link";
import {useCallback, useEffect, useState} from "react";

import {api} from "@/lib/api";

import type * as corev1 from "@/contracts/core/v1";
import type {ReactNode} from "react";

interface TeamProblemsTabProps {
  teamId: string;
}

export const TeamProblemsTab = ({teamId}: TeamProblemsTabProps): ReactNode => {
  const [problems, setProblems] = useState<corev1.ProblemsListItemModel[]>([]);
  const [loading, setLoading] = useState(true);

  const loadProblems = useCallback(async () => {
    setLoading(true);
    const [error, response] = await api.listTeamProblems({id: teamId});
    setLoading(false);

    if (error || !response) {
      console.error("Failed to load team problems:", error);
      return;
    }

    setProblems(response.problems);
  }, [teamId]);

  useEffect(() => {
    loadProblems();
  }, [loadProblems]);

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
                  href={`/${p.organization_login}/problems/${p.id}`}
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
    </Card>
  );
};
