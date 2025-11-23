"use client";

import {
  Box,
  Table,
  TableTbody,
  TableTd,
  TableTh,
  TableThead,
  TableTr,
  Text,
} from "@mantine/core";
import { useRouter } from "next/navigation";
import type { ContestModel } from "../../contracts/core/v1";

type ContestsTableProps = {
  contests: ContestModel[];
  showCreatedAt?: boolean;
};

export function ContestsTable({ contests, showCreatedAt = true }: ContestsTableProps) {
  const router = useRouter();

  return (
    <Box style={{ overflowX: "auto" }}>
      <Table striped highlightOnHover withTableBorder withColumnBorders>
      <TableThead>
        <TableTr>
          <TableTh style={{ width: "50%", maxWidth: 0 }}>Название</TableTh>
          <TableTh style={{ width: "15%"}} visibleFrom="sm">Участники</TableTh>
          <TableTh style={{ width: "15%"}} visibleFrom="sm">Задачи</TableTh>
          {showCreatedAt && <TableTh style={{ width: "20%"}}>Дата создания</TableTh>}
        </TableTr>
      </TableThead>
      <TableTbody>
        {contests.map((contest: ContestModel) => (
          <TableTr
            key={contest.id}
            onClick={() => router.push(`/contests/${contest.id}`)}
            style={{ cursor: "pointer" }}
          >
            <TableTd style={{ width: "50%", maxWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              <Text fw={600}>{contest.title}</Text>
            </TableTd>
            <TableTd visibleFrom="sm">
              <Text>{0}</Text>
            </TableTd>
            <TableTd visibleFrom="sm">
              <Text>{0}</Text>
            </TableTd>
            {showCreatedAt && (
              <TableTd>
                <Text>{new Date(contest.created_at).toLocaleDateString("ru-RU")}</Text>
              </TableTd>
            )}
          </TableTr>
        ))}
      </TableTbody>
    </Table>
    </Box>
  );
}
