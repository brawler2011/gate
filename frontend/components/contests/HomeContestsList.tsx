"use client";

import { Text, Table, Box } from "@mantine/core";
import { IconTrophy, IconChevronRight } from "@tabler/icons-react";
import { useRouter } from "next/navigation";
import type { ContestModel } from "@contracts/core/v1";
import classes from "./HomeContestsList.module.css";

type HomeContestsListProps = {
  contests: ContestModel[];
};

export function HomeContestsList({ contests }: HomeContestsListProps) {
  const router = useRouter();

  if (contests.length === 0) {
    return (
      <Text c="dimmed" ta="center" py="xl">
        У вас пока нет контестов
      </Text>
    );
  }

  return (
    <Box style={{ overflowX: "auto" }}>
      <Table 
        className={classes.table} 
        verticalSpacing="sm"
      >
        <Table.Tbody>
          {contests.map((contest) => (
            <Table.Tr
              key={contest.id}
              className={classes.row}
              onClick={() => router.push(`/contests/${contest.id}`)}
            >
              <Table.Td className={classes.iconCell}>
                <IconTrophy size={20} className={classes.trophyIcon} />
              </Table.Td>
              
              <Table.Td className={classes.infoCell}>
                <Text fw={600} lineClamp={1} size="sm">
                  {contest.title}
                </Text>
              </Table.Td>
              
              <Table.Td className={classes.dateCell}>
                {new Date(contest.created_at).toLocaleDateString("ru-RU")}
              </Table.Td>
              
              <Table.Td className={classes.arrowCell}>
                <IconChevronRight size={16} style={{ opacity: 0.5 }} />
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </Box>
  );
}
