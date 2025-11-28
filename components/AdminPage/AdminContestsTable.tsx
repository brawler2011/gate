"use client";

import {
  ActionIcon,
  Badge,
  Box,
  Group,
  Table,
  Text,
} from "@mantine/core";
import { IconEdit, IconTrash } from "@tabler/icons-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import type { ContestModel } from "../../../contracts/core/v1";
import { TruncatedWithCopy } from "../TruncatedWithCopy";
import { DeleteContestModal } from "./DeleteContestModal";
import classes from "./styles.module.css";

type AdminContestsTableProps = {
  contests: ContestModel[];
  onDeleteContest: (contestId: string) => Promise<void>;
};

function getVisibilityDisplay(visibility: string) {
  if (visibility === "public") {
    return { label: "Публичный", color: "green" };
  }
  return { label: "Приватный", color: "gray" };
}

export function AdminContestsTable({ contests, onDeleteContest }: AdminContestsTableProps) {
  const router = useRouter();
  const [deleteModalOpened, setDeleteModalOpened] = useState(false);
  const [contestToDelete, setContestToDelete] = useState<ContestModel | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const handleRowClick = (contestId: string) => {
    router.push(`/contests/${contestId}`);
  };

  const handleEditClick = (e: React.MouseEvent, contestId: string) => {
    e.stopPropagation();
    router.push(`/contests/${contestId}/manage`);
  };

  const handleAuthorClick = (e: React.MouseEvent, authorId: string) => {
    e.stopPropagation();
    router.push(`/users/${authorId}`);
  };

  const handleDeleteClick = (e: React.MouseEvent, contest: ContestModel) => {
    e.stopPropagation();
    setContestToDelete(contest);
    setDeleteModalOpened(true);
  };

  const handleDeleteConfirm = async () => {
    if (!contestToDelete) return;
    
    setDeletingId(contestToDelete.id);
    try {
      await onDeleteContest(contestToDelete.id);
    } finally {
      setDeletingId(null);
      setContestToDelete(null);
    }
  };

  return (
    <>
      <Box className={classes.tableContainer}>
        <Table className={classes.table} verticalSpacing="xs">
          <Table.Thead className={classes.thead}>
            <Table.Tr>
              <Table.Th style={{ width: "30%" }}>Название</Table.Th>
              <Table.Th style={{ width: "12%" }}>ID</Table.Th>
              <Table.Th style={{ width: "12%" }}>Видимость</Table.Th>
              <Table.Th style={{ width: "15%" }}>Автор</Table.Th>
              <Table.Th style={{ width: "15%" }}>Дата создания</Table.Th>
              <Table.Th style={{ width: "10%" }}>Действия</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody className={classes.tbody}>
            {contests.map((contest) => {
              const visibilityDisplay = getVisibilityDisplay(contest.visibility);
              return (
                <Table.Tr
                  key={contest.id}
                  onClick={() => handleRowClick(contest.id)}
                >
                  <Table.Td>
                    <Text className={classes.titleCell} lineClamp={1}>
                      {contest.title}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <TruncatedWithCopy value={contest.id} />
                  </Table.Td>
                  <Table.Td>
                    <Badge
                      variant="filled"
                      color={visibilityDisplay.color}
                      tt="none"
                      size="sm"
                    >
                      {visibilityDisplay.label}
                    </Badge>
                  </Table.Td>
                  <Table.Td>
                    <Badge
                      variant="light"
                      color="blue"
                      tt="none"
                      size="sm"
                      className={classes.authorBadge}
                      onClick={(e) => handleAuthorClick(e, contest.created_by)}
                    >
                      {contest.created_by.slice(0, 8)}
                    </Badge>
                  </Table.Td>
                  <Table.Td>
                    <Text className={classes.dateCell}>
                      {new Date(contest.created_at).toLocaleDateString("ru-RU")}
                    </Text>
                  </Table.Td>
                  <Table.Td className={classes.actionsCell}>
                    <Group gap="xs" wrap="nowrap">
                      <ActionIcon
                        color="blue"
                        variant="subtle"
                        onClick={(e) => handleEditClick(e, contest.id)}
                      >
                        <IconEdit size={16} />
                      </ActionIcon>
                      <ActionIcon
                        color="red"
                        variant="subtle"
                        onClick={(e) => handleDeleteClick(e, contest)}
                        loading={deletingId === contest.id}
                      >
                        <IconTrash size={16} />
                      </ActionIcon>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              );
            })}
          </Table.Tbody>
        </Table>
      </Box>

      {contestToDelete && (
        <DeleteContestModal
          opened={deleteModalOpened}
          onClose={() => {
            setDeleteModalOpened(false);
            setContestToDelete(null);
          }}
          contest={{
            id: contestToDelete.id,
            title: contestToDelete.title,
          }}
          onSubmit={handleDeleteConfirm}
        />
      )}
    </>
  );
}

