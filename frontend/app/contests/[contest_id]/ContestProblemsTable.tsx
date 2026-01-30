"use client";

import { numberToLetters } from "@/lib/lib";
import { CONTEST_CONTENT_MAX_WIDTH } from "@/lib/constants";
import { Box, Table, Text } from "@mantine/core";
import { useRouter } from "next/navigation";
import type { ContestProblemListItemModel } from "@contracts/core/v1";
import classes from "./ContestProblemsTable.module.css";

type ContestProblemsTableProps = {
  contestId: string | number;
  problems: Array<ContestProblemListItemModel>;
};

const formatTimeLimit = (timeMs: number) => {
  if (timeMs % 1000 === 0) {
    return `${timeMs / 1000}s`;
  }
  return `${timeMs}ms`;
};

const formatMemoryLimit = (memoryKb: number) => {
  return `${memoryKb}MB`;
};

export function ContestProblemsTable({
  contestId,
  problems,
}: ContestProblemsTableProps) {
  const router = useRouter();

  return (
    <Box style={{ maxWidth: CONTEST_CONTENT_MAX_WIDTH, margin: "0 auto" }}>
      <Box className={classes.tableContainer}>
        <Table className={classes.table} verticalSpacing="md">
          <Table.Thead className={classes.thead}>
            <Table.Tr>
              <Table.Th style={{ textAlign: "center" }}>#</Table.Th>
              <Table.Th>Задача</Table.Th>
              <Table.Th style={{ textAlign: "center" }}>Статус</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody className={classes.tbody}>
            {problems.map((problem) => {
              const problemUrl = `/contests/${contestId}/problems/${
                problem.problem_id || ""
              }`;
              return (
                <Table.Tr
                  key={problem.problem_id}
                  onClick={() => router.push(problemUrl)}
                >
                  <Table.Td className={classes.positionCell}>
                    {numberToLetters(problem.position)}
                  </Table.Td>
                  <Table.Td>
                    <Text className={classes.titleText}>
                      {problem.title}
                    </Text>
                    <Text className={classes.limitsText}>
                      {formatTimeLimit(problem.time_limit)} / {formatMemoryLimit(problem.memory_limit)}
                    </Text>
                  </Table.Td>
                  <Table.Td className={classes.scoreCell}>
                    -
                  </Table.Td>
                </Table.Tr>
              );
            })}
          </Table.Tbody>
        </Table>
      </Box>
    </Box>
  );
}
