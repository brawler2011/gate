"use client";

import { numberToLetters } from "@/lib/lib";
import { Text, Group, Table, Box } from "@mantine/core";
import { IconChevronRight } from "@tabler/icons-react";
import { useRouter } from "next/navigation";
import classes from "./HomeProblemsTable.module.css";

type HomeProblemItem = {
  id: string;
  contest_id: string;
  problem_id: string;
  position: number;
  title: string;
  time_limit: number;
  memory_limit: number;
};

type HomeProblemsTableProps = {
  problems: Array<HomeProblemItem>;
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

export function HomeProblemsTable({ problems }: HomeProblemsTableProps) {
  const router = useRouter();

  if (problems.length === 0) {
    return (
      <Text c="dimmed" ta="center" py="xl">
        Нет доступных задач
      </Text>
    );
  }

  return (
    <Box style={{ overflowX: "auto" }}>
      <Table 
        className={classes.table} 
        verticalSpacing="sm"
        withRowBorders={true}
      >
        <Table.Tbody>
          {problems.map((problem) => {
            const problemUrl = `/contests/${problem.contest_id}/problems/${problem.problem_id}`;
            return (
              <Table.Tr
                key={problem.id}
                className={classes.row}
                onClick={() => router.push(problemUrl)}
              >
                <Table.Td className={classes.positionCell}>
                  <div className={classes.badge}>{numberToLetters(problem.position)}</div>
                </Table.Td>
                <Table.Td className={classes.titleCell}>
                  <Text fw={600} size="sm">
                    {problem.title}
                  </Text>
                </Table.Td>
                <Table.Td className={classes.limitsCell}>
                  <Group gap="xs" justify="flex-end">
                    <Text size="xs" c="dimmed">
                      {formatTimeLimit(problem.time_limit)}
                    </Text>
                    <Text size="xs" c="dimmed">
                      /
                    </Text>
                    <Text size="xs" c="dimmed">
                      {formatMemoryLimit(problem.memory_limit)}
                    </Text>
                  </Group>
                </Table.Td>
                <Table.Td className={classes.iconCell}>
                  <IconChevronRight size={16} style={{ opacity: 0.5 }} />
                </Table.Td>
              </Table.Tr>
            );
          })}
        </Table.Tbody>
      </Table>
    </Box>
  );
}
