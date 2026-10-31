import {Button, Checkbox, Group, Modal, Stack, Text, Textarea} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {useQueryClient} from "@tanstack/react-query";
import React, {useState, type ReactNode} from "react";
import {useRevalidator} from "react-router-dom";

import {api} from "@/lib/api";

export type SanctionActionType =
  | "block_submission"
  | "unblock_submission"
  | "block_problem"
  | "unblock_problem";

interface SubmissionSanctionsModalProps {
  actionType: SanctionActionType | null;
  onClose: () => void;
  orgLogin?: string;
  contestLogin?: string;
  submissionId?: string;
  userId?: string;
  problemId?: string;
  username?: string;
  problemTitle?: string;
  onSuccess?: () => void;
}

export const SubmissionSanctionsModal = ({
  actionType,
  onClose,
  orgLogin,
  contestLogin,
  submissionId,
  userId,
  problemId,
  username,
  problemTitle,
  onSuccess,
}: SubmissionSanctionsModalProps): ReactNode => {
  const [reason, setReason] = useState("");
  const [rejudgeSubmissions, setRejudgeSubmissions] = useState(false);
  const [loading, setLoading] = useState(false);
  const revalidator = useRevalidator();
  const queryClient = useQueryClient();

  if (!actionType) {
    return null;
  }

  const handleConfirm = async () => {
    if (!orgLogin || !contestLogin) {
      return;
    }

    setLoading(true);
    let err: {message?: string} | null | undefined = null;

    if (actionType === "block_submission" && submissionId) {
      const {error} = await api.POST(
        "/organizations/{org_login}/contests/{contest_login}/submissions/{submission_id}/block",
        {
          params: {
            path: {
              org_login: orgLogin,
              contest_login: contestLogin,
              submission_id: submissionId,
            },
          },
          body: {
            reason: reason.trim() ? reason.trim() : undefined,
          },
        },
      );
      err = error;
    } else if (actionType === "unblock_submission" && submissionId) {
      const {error} = await api.POST(
        "/organizations/{org_login}/contests/{contest_login}/submissions/{submission_id}/unblock",
        {
          params: {
            path: {
              org_login: orgLogin,
              contest_login: contestLogin,
              submission_id: submissionId,
            },
          },
        },
      );
      err = error;
    } else if (actionType === "block_problem" && userId && problemId) {
      const {error} = await api.POST(
        "/organizations/{org_login}/contests/{contest_login}/participants/{user_id}/problems/{problem_id}/block",
        {
          params: {
            path: {
              org_login: orgLogin,
              contest_login: contestLogin,
              user_id: userId,
              problem_id: problemId,
            },
          },
          body: {
            reason: reason.trim() ? reason.trim() : undefined,
          },
        },
      );
      err = error;
    } else if (actionType === "unblock_problem" && userId && problemId) {
      const {error} = await api.DELETE(
        "/organizations/{org_login}/contests/{contest_login}/participants/{user_id}/problems/{problem_id}/block",
        {
          params: {
            path: {
              org_login: orgLogin,
              contest_login: contestLogin,
              user_id: userId,
              problem_id: problemId,
            },
            query: {
              rejudge_submissions: rejudgeSubmissions,
            },
          },
        },
      );
      err = error;
    }

    setLoading(false);

    if (err) {
      notifications.show({
        title: "Ошибка",
        message: err.message || "Не удалось применить санкцию",
        color: "red",
      });
    } else {
      let successMsg = "Действие успешно выполнено";
      if (actionType === "block_submission") {
        successMsg = "Посылка успешно заблокирована (дисквалифицирована)";
      } else if (actionType === "unblock_submission") {
        successMsg = "Посылка разблокирована и отправлена на перетестирование";
      } else if (actionType === "block_problem") {
        successMsg = "Задача успешно заблокирована для пользователя";
      } else if (actionType === "unblock_problem") {
        successMsg = "Задача успешно разблокирована для пользователя";
      }

      notifications.show({
        title: "Успешно",
        message: successMsg,
        color: "green",
      });

      setReason("");
      setRejudgeSubmissions(false);
      onClose();
      if (onSuccess) {
        onSuccess();
      }
      void queryClient.invalidateQueries({
        queryKey: api.queryOptions(
          "get",
          "/organizations/{org_login}/contests/{contest_login}/submissions",
          {
            params: {
              path: {
                org_login: orgLogin,
                contest_login: contestLogin,
              },
            },
          },
        ).queryKey,
      });
      void queryClient.invalidateQueries({
        queryKey: api.queryOptions(
          "get",
          "/organizations/{org_login}/contests/{contest_login}/scoreboard",
          {
            params: {
              path: {
                org_login: orgLogin,
                contest_login: contestLogin,
              },
            },
          },
        ).queryKey,
      });
      if (submissionId) {
        void queryClient.invalidateQueries({
          queryKey: api.queryOptions("get", "/submissions/{submission_id}", {
            params: {path: {submission_id: submissionId}},
          }).queryKey,
        });
        void queryClient.invalidateQueries({queryKey: [["submission-details", submissionId]]});
      }
      void revalidator.revalidate();
    }
  };

  let title = "";
  let description = "";
  let confirmColor = "red";
  let confirmLabel = "Подтвердить";

  switch (actionType) {
    case "block_submission":
      title = "Дисквалификация посылки";
      description = `Вы собираетесь заблокировать (дисквалифицировать) данную посылку${
        username ? ` пользователя ${username}` : ""
      }. Вердикт изменится на DQ (0 баллов, +1 штрафная попытка).`;
      confirmLabel = "Заблокировать посылку";
      confirmColor = "red";
      break;
    case "unblock_submission":
      title = "Разблокировка посылки";
      description =
        "Вы собираетесь разблокировать данную посылку. Она будет отправлена на полное повторное тестирование.";
      confirmLabel = "Разблокировать и перетестировать";
      confirmColor = "blue";
      break;
    case "block_problem":
      title = "Блокировка задачи для пользователя";
      description = `Вы собираетесь заблокировать задачу${
        problemTitle ? ` "${problemTitle}"` : ""
      } для пользователя${
        username ? ` ${username}` : ""
      }. Все существующие посылки будут дисквалифицированы (DQ), а любые будущие посылки пользователя по этой задаче будут сразу помечаться как DQ без тестирования.`;
      confirmLabel = "Заблокировать задачу";
      confirmColor = "red";
      break;
    case "unblock_problem":
      title = "Разблокировка задачи для пользователя";
      description = `Вы собираетесь снять блокировку по задаче${
        problemTitle ? ` "${problemTitle}"` : ""
      } для пользователя${
        username ? ` ${username}` : ""
      }.`;
      confirmLabel = "Разблокировать задачу";
      confirmColor = "blue";
      break;
  }

  const showReasonInput =
    actionType === "block_submission" || actionType === "block_problem";
  const showRejudgeCheckbox = actionType === "unblock_problem";

  return (
    <Modal opened={Boolean(actionType)} onClose={onClose} title={title} centered>
      <Stack gap="md">
        <Text size="sm">{description}</Text>

        {showReasonInput && (
          <Textarea
            label="Причина блокировки (необязательно)"
            placeholder="Например: Списывание / GPT-код / Обмен решениями"
            value={reason}
            onChange={(e) => setReason(e.currentTarget.value)}
            rows={3}
          />
        )}

        {showRejudgeCheckbox && (
          <Checkbox
            label="Отправить все ранее заблокированные посылки на перетестирование"
            description="Если флаг выключен, старые посылки останутся DQ, но новые будут тестироваться нормально."
            checked={rejudgeSubmissions}
            onChange={(e) => setRejudgeSubmissions(e.currentTarget.checked)}
          />
        )}

        <Group justify="flex-end" gap="xs" mt="sm">
          <Button variant="default" onClick={onClose} disabled={loading}>
            Отмена
          </Button>
          <Button color={confirmColor} onClick={handleConfirm} loading={loading}>
            {confirmLabel}
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
};
