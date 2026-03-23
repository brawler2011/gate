"use client";
import type { ProblemsListItemModel } from '@contracts/gateway/v1';
import { Table, Text, Anchor, Badge } from '@mantine/core';
import Link from 'next/link';

type Props = { problems: ProblemsListItemModel[] };

export function OrgProblemsTab({ problems }: Props) {
  if (problems.length === 0) {
    return <Text c="dimmed" py="xl" ta="center">Задачи не найдены</Text>;
  }
  return (
    <Table verticalSpacing="sm">
      <Table.Thead>
        <Table.Tr>
          <Table.Th>Название</Table.Th>
          <Table.Th>Лимит памяти</Table.Th>
          <Table.Th>Лимит времени</Table.Th>
          <Table.Th>Создана</Table.Th>
        </Table.Tr>
      </Table.Thead>
      <Table.Tbody>
        {problems.map((p) => (
          <Table.Tr key={p.id}>
            <Table.Td>
              <Anchor component={Link} href={`/problems/${p.id}/workshop`} size="sm">
                {p.title}
              </Anchor>
            </Table.Td>
            <Table.Td>
              <Badge variant="light" color="blue" size="sm">{p.memory_limit} МБ</Badge>
            </Table.Td>
            <Table.Td>
              <Badge variant="light" color="teal" size="sm">{p.time_limit} мс</Badge>
            </Table.Td>
            <Table.Td>
              <Text size="sm" c="dimmed">
                {new Date(p.created_at).toLocaleDateString('ru-RU')}
              </Text>
            </Table.Td>
          </Table.Tr>
        ))}
      </Table.Tbody>
    </Table>
  );
}
