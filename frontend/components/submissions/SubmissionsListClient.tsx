"use client";

import {Button, Group, Modal, Text} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconRefresh} from "@tabler/icons-react";
import React, {useState, type ReactNode} from "react";
import {useLocation} from "react-router-dom";

import {RoutePagination} from "@/components/shared/Pagination";
import {QueryState} from "@/components/shared/QueryState";
import {useSubmissions} from "@/hooks/useSubmissions";
import {api} from "@/lib/api";

import {SubmissionsList} from "./SubmissionsList";

import type {SubmissionsSelection} from "@/lib/submissions";

interface SubmissionsListClientProps {
  selection: SubmissionsSelection;
  canRejudge?: boolean;
}

type ModalState =
  | { type: "submission"; id: string }
  | { type: "problem"; problemId: string }
  | { type: "all" }
  | null;

export const SubmissionsListClient = ({
  selection,
  canRejudge = false,
}: SubmissionsListClientProps): ReactNode => {
  const {submissions, highlightedIds, data, error} = useSubmissions(selection);
  const filter = selection;
  const location = useLocation();

  const [modalState, setModalState] = useState<ModalState>(null);
  const [loading, setLoading] = useState(false);
  const [rejudgingId, setRejudgingId] = useState<string | null>(null);

  if (!data) {
    return <QueryState error={error} />;
  }

  const handleRejudgeConfirm = async () => {
    if (!modalState || !filter.orgLogin || !filter.contestLogin) {
      return;
    }

    setLoading(true);
    let err: {message?: string} | null | undefined = null;

    if (modalState.type === "submission") {
      setRejudgingId(modalState.id);
      const {error} = await api.POST(
        "/organizations/{org_login}/contests/{contest_login}/submissions/{submission_id}/rejudge",
        {
          params: {
            path: {
              org_login: filter.orgLogin,
              contest_login: filter.contestLogin,
              submission_id: modalState.id,
            },
          },
        },
      );
      err = error;
    } else if (modalState.type === "problem") {
      const {error} = await api.POST(
        "/organizations/{org_login}/contests/{contest_login}/problems/{problem_id}/rejudge",
        {
          params: {
            path: {
              org_login: filter.orgLogin,
              contest_login: filter.contestLogin,
              problem_id: modalState.problemId,
            },
          },
        },
      );
      err = error;
    } else if (modalState.type === "all") {
      const {error} = await api.POST(
        "/organizations/{org_login}/contests/{contest_login}/rejudge",
        {
          params: {
            path: {
              org_login: filter.orgLogin,
              contest_login: filter.contestLogin,
            },
          },
        },
      );
      err = error;
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

      <Group justify="center">
        <RoutePagination pagination={data.pagination} baseUrl={location.pathname} queryParams={{
          page: selection.page, pageSize: selection.pageSize, userId: selection.userId,
          problemId: selection.problemId, state: selection.state,
          language: selection.language, order: selection.sortOrder,
        }} />
      </Group>

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
