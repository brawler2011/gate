"use client";
import type { ContestModel } from '@contracts/gateway/v1';
import { Table, Text, Anchor, Badge } from '@mantine/core';
import Link from 'next/link';

const VISIBILITY_LABELS: Record<string, string> = {
  public: 'Публичный',
  private: 'Приватный',
  unlisted: 'Скрытый',
};
const VISIBILITY_COLORS: Record<string, string> = {
  public: 'green',
  private: 'red',
  unlisted: 'gray',
};

type Props = { contests: ContestModel[] };

export function OrgContestsTab({ contests }: Props) {
  if (contests.length === 0) {
    return <Text c="dimmed" py="xl" ta="center">Контесты не найдены</Text>;
  }
  return (
    <Table verticalSpacing="sm">
      <Table.Thead>
        <Table.Tr>
          <Table.Th>Название</Table.Th>
          <Table.Th>Видимость</Table.Th>
          <Table.Th>Создан</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {contests.map((c) => (
          <Table.Tr key={c.id}>
            <Table.Td>
              <Anchor component={Link} href={`/contests/${c.id}`} size="sm">
                {c.title}
              </Anchor>
            </Table.Td>
            <Table.Td>
              <Badge
                variant="light"
                color={VISIBILITY_COLORS[c.visibility] ?? 'gray'}
                size="sm"
              >
                {VISIBILITY_LABELS[c.visibility] ?? c.visibility}
              </Badge>
            </Table.Td>
            <Table.Td>
              <Text size="sm" c="dimmed">
                {new Date(c.created_at).toLocaleDateString('ru-RU')}
              </Text>
            </Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  );
}
