"use client";

import {Button, Group, Modal, Text} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconRefresh} from "@tabler/icons-react";
import React, {useState, type ReactNode} from "react";

import {api} from "@/lib/api";
import {useSubmissionsWebSocket} from "@/lib/useSubmissionsWebSocket";

import {SubmissionsList} from "./SubmissionsList";

import type {SubmissionsListItemModel} from "@/contracts/core/v1";

interface SubmissionsListClientProps {
  initialSubmissions: SubmissionsListItemModel[];
  wsUrl: string;
  since?: number;
  snapshotScope?: "all" | "mine";
  filter: {
    orgLogin?: string;
    contestLogin?: string;
    userId?: string;
    problemId?: string;
  };
  pageSize: number;
  page: number;
  sortOrder?: string;
  canRejudge?: boolean;
}

type ModalState =
  | { type: "submission"; id: string }
  | { type: "problem"; problemId: string }
  | { type: "all" }
  | null;

export const SubmissionsListClient = ({
  initialSubmissions,
  wsUrl,
  since,
  snapshotScope = "all",
  filter,
  pageSize,
  page,
  sortOrder,
  canRejudge = false,
}: SubmissionsListClientProps): ReactNode => {
  const enabled = page === 1 && (sortOrder === "desc" || sortOrder === undefined);

  const {submissions, highlightedIds} = useSubmissionsWebSocket({
    wsUrl,
    since,
    initialSubmissions,
    snapshotScope,
    filter,
    pageSize,
    enabled,
  });

  const [modalState, setModalState] = useState<ModalState>(null);
  const [loading, setLoading] = useState(false);
  const [rejudgingId, setRejudgingId] = useState<string | null>(null);

  const handleRejudgeConfirm = async () => {
    if (!modalState || !filter.orgLogin || !filter.contestLogin) {
      return;
    }

    setLoading(true);
    let err = null;

    if (modalState.type === "submission") {
      setRejudgingId(modalState.id);
      [err] = await api.rejudgeSubmission({
        orgLogin: filter.orgLogin,
        contestLogin: filter.contestLogin,
        submissionId: modalState.id,
      });
    } else if (modalState.type === "problem") {
      [err] = await api.rejudgeContestProblem({
        orgLogin: filter.orgLogin,
        contestLogin: filter.contestLogin,
        problemId: modalState.problemId,
      });
    } else if (modalState.type === "all") {
      [err] = await api.rejudgeContest({
        orgLogin: filter.orgLogin,
        contestLogin: filter.contestLogin,
      });
    }

    setLoading(false);
    setRejudgingId(null);
    setModalState(null);

    if (err) {
      notifications.show({
        title: "Ошибка",
        message: err.message || "Не удалось отправить решения на перетестирование",
        color: "red",
      });
    } else {
      notifications.show({
        title: "Успешно",
        message: "Решения отправлены на повторную проверку",
        color: "green",
      });
    }
  };

  let modalTitle = "Перетестировать все решения контеста";
  if (modalState?.type === "submission") {
    modalTitle = "Перетестирование посылки";
  } else if (modalState?.type === "problem") {
    modalTitle = "Перетестировать все решения задачи";
  }

  let modalDescription = "Вы действительно хотите отправить ВСЕ решения контеста на повторную проверку?";
  if (modalState?.type === "submission") {
    modalDescription = "Вы действительно хотите отправить выбранное решение на повторную проверку?";
  } else if (modalState?.type === "problem") {
    modalDescription = "Вы действительно хотите отправить ВСЕ решения этой задачи в контесте на повторную проверку?";
  }

  return (
    <>
      {canRejudge && filter.orgLogin && filter.contestLogin && (
        <Group justify="flex-end" mb="xs">
          {filter.problemId && (
            <Button
              size="xs"
              variant="outline"
              color="orange"
              leftSection={<IconRefresh size="0.9rem" />}
              onClick={() =>
                setModalState({type: "problem", problemId: filter.problemId!})
              }
            >
              Перетестировать задачу
            </Button>
          )}
          <Button
            size="xs"
            variant="outline"
            color="red"
            leftSection={<IconRefresh size="0.9rem" />}
            onClick={() => setModalState({type: "all"})}
          >
            Перетестировать все решения
          </Button>
        </Group>
      )}

      <SubmissionsList
        submissions={submissions}
        highlightedIds={highlightedIds}
        canRejudge={canRejudge}
        onRejudgeSubmission={async (submissionId) => {
          setModalState({type: "submission", id: submissionId});
        }}
        rejudgingId={rejudgingId}
      />

      <Modal
        opened={modalState !== null}
        onClose={() => setModalState(null)}
        title={modalTitle}
        centered
      >
        <Text size="sm" mb="lg">
          {modalDescription}
        </Text>
        <Group justify="flex-end">
          <Button
            variant="default"
            onClick={() => setModalState(null)}
            disabled={loading}
          >
            Отмена
          </Button>
          <Button
            color="red"
            onClick={handleRejudgeConfirm}
            loading={loading}
          >
            Перетестировать
          </Button>
        </Group>
      </Modal>
    </>
  );
};
