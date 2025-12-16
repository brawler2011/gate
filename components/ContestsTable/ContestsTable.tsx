"use client";

import {
  Box,
  Table,
  Text,
} from "@mantine/core";
import { useRouter } from "next/navigation";
import type { ContestModel } from "@contracts/core/v1";
import classes from "./styles.module.css";

type ContestsTableProps = {
  contests: ContestModel[];
  showCreatedAt?: boolean;
};

export function ContestsTable({ contests, showCreatedAt = true }: ContestsTableProps) {
  const router = useRouter();

  return (
    <Box className={classes.tableContainer}>
      <Table className={classes.table} verticalSpacing="sm">
        <Table.Thead className={classes.thead}>
          <Table.Tr>
            <Table.Th style={{ width: "40%" }}>Название</Table.Th>
            <Table.Th style={{ width: "20%" }}>Участники</Table.Th>
            <Table.Th style={{ width: "20%" }}>Задачи</Table.Th>
            {showCreatedAt && <Table.Th style={{ width: "20%" }}>Дата создания</Table.Th>}
          </Table.Tr>
        </Table.Thead>
        <Table.Tbody className={classes.tbody}>
          {contests.map((contest: ContestModel) => (
            <Table.Tr
              key={contest.id}
              onClick={() => router.push(`/contests/${contest.id}`)}
            >
              <Table.Td>
                <Text className={classes.titleCell} lineClamp={1}>
                  {contest.title}
                </Text>
              </Table.Td>
              <Table.Td>
                <Text className={classes.metaCell}>0</Text>
              </Table.Td>
              <Table.Td>
                <Text className={classes.metaCell}>0</Text>
              </Table.Td>
              {showCreatedAt && (
                <Table.Td>
                  <Text className={classes.metaCell}>
                    {new Date(contest.created_at).toLocaleDateString("ru-RU")}
                  </Text>
                </Table.Td>
              )}
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Box>
  );
}

