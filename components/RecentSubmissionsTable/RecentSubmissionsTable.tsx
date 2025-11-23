"use client";

import { Anchor, Table, Text } from "@mantine/core";
import Link from "next/link";
import type { SubmissionsListItemModel } from "../../../contracts/core/v1";
import { StateColor, StateString } from "@/lib/lib";

type RecentSubmissionsTableProps = {
  submissions: SubmissionsListItemModel[];
  contestId: string;
  userId?: string;
};

export function RecentSubmissionsTable({
  submissions,
  contestId,
  userId,
}: RecentSubmissionsTableProps) {
  if (submissions.length === 0) {
    return null;
  }

  return (
    <>
      <Text fw={500}>
        Последние посылки{" "}
        <Anchor
          component={Link}
          href={`/mysubmissions?contestId=${contestId}&sortOrder=desc&userId=${userId}`}
          fs="italic"
          c="var(--mantine-color-text)"
          fw={500}
        >
          (посмотреть все)
        </Anchor>
        :
      </Text>
      <Table verticalSpacing="xs" horizontalSpacing="sm">
        <Table.Thead>
          <Table.Tr>
            <Table.Th ta="center">Задача</Table.Th>
            <Table.Th ta="center">Статус</Table.Th>
            <Table.Th ta="center">Очки</Table.Th>
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody>
          {submissions.slice(0, 5).map((submission) => (
            <Table.Tr key={submission.id}>
              <Table.Td ta="center">
                <Text fw={500}>{submission.problem_title}</Text>
              </Table.Td>
              <Table.Td ta="center">
                <Text c={StateColor(submission.state)} fw={500}>
                  {StateString(submission.state)}
                </Text>
              </Table.Td>
              <Table.Td ta="center">{submission.score}</Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </>
  );
}
