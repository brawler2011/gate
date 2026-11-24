"use client";

import {Alert, Button, Group, Modal, Stack, Text, Textarea} from "@mantine/core";
import {useDisclosure} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {IconCheck, IconInfoCircle, IconTrophy, IconX} from "@tabler/icons-react";
import {useQueryClient} from "@tanstack/react-query";
import {useState, type ReactNode} from "react";

import {contestQueryKey} from "@/hooks/useContestData";
import {dispatchNotificationsUpdated} from "@/hooks/useUnreadNotificationsCount";
import {api} from "@/lib/api";
import {problemMessage} from "@/lib/problemText";

import type {ContestJoinRequestModel, ContestModel} from "@/contracts/core/v1/schema";

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
  const client = useQueryClient();
  const {data: details, isLoading: loading} = api.useQuery("get", "/organizations/{org_login}/contests/{contest_login}", {
    params: {path: {org_login: orgLogin, contest_login: contest.login}},
  }, {enabled: isAuthenticated && !hasAccess});
  const pendingRequest = details?.my_join_request ?? null;
  const refresh = () => client.invalidateQueries({queryKey: contestQueryKey(orgLogin, contest.login)});
  const [actionLoading, setActionLoading] = useState(false);
  const [modalOpened, {open: openModal, close: closeModal}] = useDisclosure(false);
  const [message, setMessage] = useState("");

  if (!isAuthenticated || hasAccess) {
    return null;
  }

  if (contest.join_policy !== "by_request") {
    return null;
  }

  const handleRequestSubmit = async () => {
    setActionLoading(true);
    const {data, error} = await api.POST(
      "/contests/{contest_id}/requests",
      {
        params: {path: {contest_id: contest.id}},
        body: {message: message.trim() || undefined},
      },
    );
    setActionLoading(false);
    closeModal();
    setMessage("");

    if (error) {
      notifications.show({
        title: "Ошибка",
        message: problemMessage(error, "Не удалось вступить в контест"),
        color: "red",
      });
      return;
    }

    if (data?.request) {
      notifications.show({
        message: "Заявка на участие отправлена модераторам контеста",
        color: "blue",
      });
      void refresh();
      dispatchNotificationsUpdated();
    }
  };

  const handleCancelRequest = async (request: ContestJoinRequestModel) => {
    setActionLoading(true);
    const {error} = await api.DELETE("/contests/{contest_id}/requests/{id}", {
      params: {path: {contest_id: contest.id, id: request.id}},
    });
    setActionLoading(false);
    if (error) {
      notifications.show({
        title: "Ошибка",
        message: problemMessage(error, "Не удалось отменить заявку"),
        color: "red",
      });
      return;
    }
    notifications.show({
      message: "Заявка на участие отменена",
      color: "blue",
    });
    void refresh();
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
            onClick={() => handleCancelRequest(pendingRequest)}
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
