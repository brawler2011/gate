"use client";

import {
  ActionIcon,
  Badge,
  Box,
  Group,
  Table,
  Text,
  Tooltip,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {
  IconDeviceDesktop,
  IconEdit,
  IconFileCode,
  IconRefresh,
  IconTrash,
} from "@tabler/icons-react";
import {useRouter} from "next/navigation";
import {useState} from "react";

import {TruncatedWithCopy} from '@/components/shared/TruncatedWithCopy';
import {api} from "@/lib/api";

import classes from "./AdminPage.module.css";
import {DeleteContestModal} from "./DeleteContestModal";

import type {ContestModel} from "@/contracts/core/v1";
import type {ReactNode} from "react";

type AdminContestsTableProps = {
  contests: ContestModel[];
  onDeleteContest: (contestId: string) => Promise<void>;
};

const getVisibilityDisplay = (visibility: string) => {
  if (visibility === "public") {
    return {label: "Публичный", color: "green"};
  }
  return {label: "Приватный", color: "gray"};
};

export const AdminContestsTable = ({contests, onDeleteContest}: AdminContestsTableProps): ReactNode => {
  const router = useRouter();
  const [deleteModalOpened, setDeleteModalOpened] = useState(false);
  const [contestToDelete, setContestToDelete] = useState<ContestModel | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const [rejudgingId, setRejudgingId] = useState<string | null>(null);

  const handleRowClick = (contestId: string) => {
    router.push(`/contests/${contestId}`);
  };

  const handleEditClick = (e: React.MouseEvent, contestId: string) => {
    e.stopPropagation();
    router.push(`/contests/${contestId}/settings`);
  };

  const handleSubmissionsClick = (e: React.MouseEvent, contestId: string) => {
    e.stopPropagation();
    router.push(`/admin/submissions?contestId=${contestId}`);
  };

  const handleMonitorClick = (e: React.MouseEvent, contestId: string) => {
    e.stopPropagation();
    router.push(`/contests/${contestId}/monitor`);
  };

  const handleRejudgeContest = async (e: React.MouseEvent, contestId: string) => {
    e.stopPropagation();
    setRejudgingId(contestId);
    try {
      const [err] = await api.rejudgeContest({contestId});
      if (err) {
        notifications.show({
          title: "Ошибка",
          message: err.message || "Не удалось запустить перепроверку контеста",
          color: "red",
        });
        return;
      }
      notifications.show({
        title: "Успех",
        message: "Перепроверка всех посылок контеста запущена",
        color: "green",
      });
    } finally {
      setRejudgingId(null);
    }
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
    if (!contestToDelete) {
      return;
    }
    
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
              <Table.Th style={{width: "25%"}}>Название</Table.Th>
              <Table.Th style={{width: "12%"}}>ID</Table.Th>
              <Table.Th style={{width: "12%"}}>Видимость</Table.Th>
              <Table.Th style={{width: "15%"}}>Автор</Table.Th>
              <Table.Th style={{width: "15%"}}>Дата создания</Table.Th>
              <Table.Th style={{width: "21%"}}>Действия</Table.Th>
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
                      <Tooltip label="Посылки контеста">
                        <ActionIcon
                          color="indigo"
                          variant="subtle"
                          onClick={(e) => handleSubmissionsClick(e, contest.id)}
                        >
                          <IconFileCode size={16} />
                        </ActionIcon>
                      </Tooltip>
                      <Tooltip label="Монитор результатов">
                        <ActionIcon
                          color="teal"
                          variant="subtle"
                          onClick={(e) => handleMonitorClick(e, contest.id)}
                        >
                          <IconDeviceDesktop size={16} />
                        </ActionIcon>
                      </Tooltip>
                      <Tooltip label="Перепроверить все посылки контеста">
                        <ActionIcon
                          color="orange"
                          variant="subtle"
                          loading={rejudgingId === contest.id}
                          onClick={(e) => handleRejudgeContest(e, contest.id)}
                        >
                          <IconRefresh size={16} />
                        </ActionIcon>
                      </Tooltip>
                      <Tooltip label="Редактировать контест">
                        <ActionIcon
                          color="blue"
                          variant="subtle"
                          onClick={(e) => handleEditClick(e, contest.id)}
                        >
                          <IconEdit size={16} />
                        </ActionIcon>
                      </Tooltip>
                      <Tooltip label="Удалить контест">
                        <ActionIcon
                          color="red"
                          variant="subtle"
                          onClick={(e) => handleDeleteClick(e, contest)}
                          loading={deletingId === contest.id}
                        >
                          <IconTrash size={16} />
                        </ActionIcon>
                      </Tooltip>
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
};
