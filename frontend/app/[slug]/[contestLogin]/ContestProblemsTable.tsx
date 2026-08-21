"use client";

import {ActionIcon, Box, Table, Text, Tooltip} from "@mantine/core";
import {IconEdit} from "@tabler/icons-react";
import {useRouter} from "next/navigation";

import {CONTEST_CONTENT_MAX_WIDTH} from "@/lib/constants";
import {numberToLetters} from "@/lib/lib";

import classes from "./ContestProblemsTable.module.css";

import type {ContestProblemListItemModel} from "@/contracts/core/v1";
import type {ReactNode} from "react";

type ContestProblemsTableProps = {
  contestLogin: string;
  orgLogin: string;
  problems: Array<ContestProblemListItemModel>;
  isManager?: boolean;
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

export const ContestProblemsTable = ({
  contestLogin,
  orgLogin,
  problems,
  isManager,
}: ContestProblemsTableProps): ReactNode => {
  const router = useRouter();

  return (
    <Box style={{width: "100%", maxWidth: CONTEST_CONTENT_MAX_WIDTH, margin: "0 auto"}}>
      <Box className={classes.tableContainer}>
        <Table className={classes.table} verticalSpacing="md">
          <Table.Thead className={classes.thead}>
            <Table.Tr>
              <Table.Th style={{textAlign: "center"}}>#</Table.Th>
              <Table.Th>Задача</Table.Th>
              <Table.Th style={{textAlign: "center"}}>Статус</Table.Th>
              {isManager && (
                <Table.Th style={{textAlign: "center", width: 80}}>Действия</Table.Th>
              )}
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody className={classes.tbody}>
            {problems.map((problem) => {
              const problemUrl = `/${orgLogin}/${contestLogin}/${numberToLetters(problem.position)}`;

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
                  {isManager && (
                    <Table.Td style={{textAlign: "center"}}>
                      <Tooltip label="Редактировать задачу" withArrow>
                        <ActionIcon
                          variant="subtle"
                          color="gray"
                          component="a"
                          href={`/${orgLogin}/problems/${problem.problem_id}`}
                          target="_blank"
                          rel="noopener noreferrer"
                          onClick={(e) => e.stopPropagation()}
                        >
                          <IconEdit size={16} />
                        </ActionIcon>
                      </Tooltip>
                    </Table.Td>
                  )}
                </Table.Tr>
              );
            })}
          </Table.Tbody>
        </Table>
      </Box>
    </Box>
  );
};
