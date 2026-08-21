"use client";

import {
  Button,
  Checkbox,
  Group,
  Modal,
  Stack,
  Text,
  Textarea,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {useRouter} from "next/navigation";
import React, {useState, type ReactNode} from "react";

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
  const router = useRouter();

  if (!actionType) {
    return null;
  }

  const handleConfirm = async () => {
    if (!orgLogin || !contestLogin) {
      return;
    }

    setLoading(true);
    let err = null;

    if (actionType === "block_submission" && submissionId) {
      [err] = await api.blockSubmission({
        orgLogin,
        contestLogin,
        submissionId,
        requestBody: {
          reason: reason.trim() ? reason.trim() : undefined,
        },
      });
    } else if (actionType === "unblock_submission" && submissionId) {
      [err] = await api.unblockSubmission({
        orgLogin,
        contestLogin,
        submissionId,
      });
    } else if (actionType === "block_problem" && userId && problemId) {
      [err] = await api.blockProblemForUser({
        orgLogin,
        contestLogin,
        userId,
        problemId,
        requestBody: {
          reason: reason.trim() ? reason.trim() : undefined,
        },
      });
    } else if (actionType === "unblock_problem" && userId && problemId) {
      [err] = await api.unblockProblemForUser({
        orgLogin,
        contestLogin,
        userId,
        problemId,
        rejudgeSubmissions,
      });
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
      router.refresh();
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
