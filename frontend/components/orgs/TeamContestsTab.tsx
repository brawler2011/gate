"use client";

import {Badge, Card, Center, Loader, Stack, Table, Text} from "@mantine/core";
import {IconTrophy} from "@tabler/icons-react";
import Link from "next/link";
import {useCallback, useEffect, useState} from "react";

import {api} from "@/lib/api";

import type * as corev1 from "@/contracts/core/v1";
import type {ReactNode} from "react";

interface TeamContestsTabProps {
  teamId: string;
}

export const TeamContestsTab = ({teamId}: TeamContestsTabProps): ReactNode => {
  const [contests, setContests] = useState<corev1.ContestModel[]>([]);
  const [loading, setLoading] = useState(true);

  const loadContests = useCallback(async () => {
    setLoading(true);
    const [error, response] = await api.listTeamContests({id: teamId});
    setLoading(false);

    if (error || !response) {
      console.error("Failed to load team contests:", error);
      return;
    }

    setContests(response.contests);
  }, [teamId]);

  useEffect(() => {
    loadContests();
  }, [loadContests]);

  if (loading) {
    return (
      <Center py="xl">
        <Loader size="md" />
      </Center>
    );
  }

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
            <Table.Th style={{textAlign: "center"}}>Видимость</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {contests.map((c) => (
            <Table.Tr key={c.id}>
              <Table.Td>
                <Text
                  component={Link}
                  href={`/${c.organization_login}/contests/${c.id}`}
                  size="sm"
                  fw={500}
                  c="blue"
                  style={{textDecoration: "none"}}
                >
                  {c.title}
                </Text>
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
