"use client";
import type { TeamModel } from '@contracts/core/v1';
import { Table, Text, Anchor } from '@mantine/core';
import Link from 'next/link';

type Props = { teams: TeamModel[]; orgId: string };

export function OrgTeamsTab({ teams, orgId }: Props) {
  if (teams.length === 0) {
    return <Text c="dimmed" py="xl" ta="center">Команды не найдены</Text>;
  }
  return (
    <Table verticalSpacing="sm">
      <Table.Thead>
        <Table.Tr>
          <Table.Th>Название</Table.Th>
          <Table.Th>Описание</Table.Th>
          <Table.Th>Создана</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {teams.map((t) => (
          <Table.Tr key={t.id}>
            <Table.Td>
              <Anchor component={Link} href={`/orgs/${orgId}/teams/${t.id}`} size="sm">
                {t.name}
              </Anchor>
            </Table.Td>
            <Table.Td><Text size="sm" c="dimmed">{t.description ?? '—'}</Text></Table.Td>
            <Table.Td>
              <Text size="sm" c="dimmed">
                {new Date(t.created_at).toLocaleDateString('ru-RU')}
              </Text>
            </Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  );
}
