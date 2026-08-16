"use client";

import {Anchor, Center, Group, Input, Stack, Table, Text} from "@mantine/core";
import {IconSearch} from "@tabler/icons-react";
import Link from "next/link";
import {useState} from "react";

import {CreateTeamModal} from "./CreateTeamModal";

import type {TeamModel} from "@/contracts/core/v1";
import type {ReactNode} from "react";

type Props = {
  teams: TeamModel[];
  orgId: string;
  canManage?: boolean;
};

export const OrgTeamsTab = ({teams, orgId, canManage = true}: Props): ReactNode => {
  const [searchQuery, setSearchQuery] = useState("");

  const filteredTeams = teams.filter((t) =>
    t.name.toLowerCase().includes(searchQuery.toLowerCase().trim()),
  );

  return (
    <Stack gap="md">
      <Group justify="space-between" align="center">
        <Input
          placeholder="Поиск команд..."
          leftSection={<IconSearch size={16} />}
          value={searchQuery}
          onChange={(event) => setSearchQuery(event.currentTarget.value)}
          radius="md"
          size="md"
          style={{flex: 1}}
        />
        {canManage && <CreateTeamModal orgId={orgId} />}
      </Group>

      {filteredTeams.length === 0 ? (
        <Center py="xl">
          <Text c="dimmed">
            {searchQuery ? "Команды по вашему запросу не найдены" : "Команды не найдены"}
          </Text>
        </Center>
      ) : (
        <Table verticalSpacing="sm" highlightOnHover>
          <Table.Thead>
            <Table.Tr>
              <Table.Th>Название</Table.Th>
              <Table.Th>Описание</Table.Th>
              <Table.Th>Создана</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody>
            {filteredTeams.map((t) => (
              <Table.Tr key={t.id}>
                <Table.Td>
                  <Anchor component={Link} href={`/orgs/${orgId}/teams/${t.id}`} size="sm" fw={500}>
                    {t.name}
                  </Anchor>
                </Table.Td>
                <Table.Td>
                  <Text size="sm" c="dimmed">
                    {t.description ?? "—"}
                  </Text>
                </Table.Td>
                <Table.Td>
                  <Text size="sm" c="dimmed">
                    {new Date(t.created_at).toLocaleDateString("ru-RU")}
                  </Text>
                </Table.Td>
              </Table.Tr>
            ))}
          </Table.Tbody>
        </Table>
      )}
    </Stack>
  );
};

