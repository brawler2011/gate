"use client";

import {
  Alert,
  Button,
  Group,
  Modal,
  Stack,
  Text,
  Textarea,
} from "@mantine/core";
import {useDisclosure} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {IconCheck, IconInfoCircle, IconTrophy, IconX} from "@tabler/icons-react";
import {useCallback, useEffect, useState, type ReactNode} from "react";

import {dispatchNotificationsUpdated} from "@/hooks/useUnreadNotificationsCount";
import {api} from "@/lib/api";

import type {ContestJoinRequestModel, ContestModel} from "@/contracts/core/v1";

type Props = {
  contest: ContestModel;
  orgLogin: string;
  isAuthenticated: boolean;
  hasAccess?: boolean;
};

export const ContestJoinButton = ({
  contest,
  orgLogin,
  isAuthenticated,
  hasAccess = false,
}: Props): ReactNode => {
  const [pendingRequest, setPendingRequest] = useState<ContestJoinRequestModel | null>(null);
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [modalOpened, {open: openModal, close: closeModal}] = useDisclosure(false);
  const [message, setMessage] = useState("");

  const checkPendingRequest = useCallback(async () => {
    if (!isAuthenticated || hasAccess) {
      return;
    }
    setLoading(true);
    const [, data] = await api.getMyContestJoinRequest({
      orgLogin,
      contestLogin: contest.login,
    });
    setLoading(false);
    if (data?.request) {
      setPendingRequest(data.request);
    } else {
      setPendingRequest(null);
    }
  }, [isAuthenticated, hasAccess, orgLogin, contest.login]);

  useEffect(() => {
    checkPendingRequest();
  }, [checkPendingRequest]);

  if (!isAuthenticated || hasAccess) {
    return null;
  }

  if (contest.participation_mode !== "by_request") {
    return null;
  }

  const handleRequestSubmit = async () => {
    setActionLoading(true);
    const [error, data] = await api.createContestJoinRequest({
      orgLogin,
      contestLogin: contest.login,
      requestBody: {
        message: message.trim() || undefined,
      },
    });
    setActionLoading(false);
    closeModal();
    setMessage("");

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message,
        color: "red",
      });
      return;
    }

    if (data?.request) {
      notifications.show({
        message: "Заявка на участие отправлена модераторам контеста",
        color: "blue",
      });
      setPendingRequest(data.request);
      dispatchNotificationsUpdated();
    }
  };

  const handleCancelRequest = async () => {
    setActionLoading(true);
    const [error] = await api.cancelContestJoinRequest({
      orgLogin,
      contestLogin: contest.login,
    });
    setActionLoading(false);
    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message,
        color: "red",
      });
      return;
    }
    notifications.show({
      message: "Заявка на участие отменена",
      color: "blue",
    });
    setPendingRequest(null);
    dispatchNotificationsUpdated();
  };

  if (pendingRequest) {
    return (
      <Alert
        icon={<IconInfoCircle size={16} />}
        title="Участие по заявкам"
        color="orange"
        variant="light"
        mb="md"
      >
        <Group justify="space-between" align="center" wrap="wrap">
          <Text size="sm">
            Ваша заявка на участие в контесте находится на рассмотрении модераторов.
          </Text>
          <Button
            size="xs"
            variant="subtle"
            color="red"
            leftSection={<IconX size={14} />}
            onClick={handleCancelRequest}
            loading={actionLoading}
          >
            Отменить заявку
          </Button>
        </Group>
      </Alert>
    );
  }

  return (
    <>
      <Alert
        icon={<IconTrophy size={16} />}
        title="Участие по предварительной регистрации"
        color="blue"
        variant="light"
        mb="md"
      >
        <Group justify="space-between" align="center" wrap="wrap">
          <Text size="sm">
            Для отправки решений в этом контесте необходимо подать заявку на участие.
          </Text>
          <Button
            size="sm"
            color="blue"
            variant="filled"
            leftSection={<IconTrophy size={16} />}
            onClick={openModal}
            loading={loading}
          >
            Подать заявку на участие
          </Button>
        </Group>
      </Alert>

      <Modal
        opened={modalOpened}
        onClose={closeModal}
        title={`Заявка на участие в контесте: ${contest.title}`}
        centered
      >
        <Stack gap="md">
          <Text size="sm" c="dimmed">
            Модераторы контеста рассмотрят вашу заявку. Вы можете указать комментарий или информацию о себе.
          </Text>
          <Textarea
            label="Комментарий к заявке (необязательно)"
            placeholder="Например: Студент 2 курса, факультет ИТ"
            autosize
            minRows={2}
            maxRows={4}
            value={message}
            onChange={(e) => setMessage(e.currentTarget.value)}
          />
          <Group justify="flex-end" mt="md">
            <Button variant="default" onClick={closeModal}>
              Отмена
            </Button>
            <Button
              color="blue"
              onClick={handleRequestSubmit}
              loading={actionLoading}
              leftSection={<IconCheck size={16} />}
            >
              Отправить заявку
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
};
