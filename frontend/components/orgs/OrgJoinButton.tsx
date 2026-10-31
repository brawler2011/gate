import {Badge, Button, Group, Modal, Stack, Text, Textarea} from "@mantine/core";
import {useDisclosure} from "@mantine/hooks";
import {notifications} from "@mantine/notifications";
import {IconCheck, IconUserPlus, IconX} from "@tabler/icons-react";
import {useQueryClient} from "@tanstack/react-query";
import {useCallback, useEffect, useState, type ReactNode} from "react";
import {useRevalidator} from "react-router-dom";

import {dispatchNotificationsUpdated} from "@/hooks/useUnreadNotificationsCount";
import {api} from "@/lib/api";

import type {OrganizationJoinRequestModel, OrganizationModel} from "@/contracts/core/v1/models";

type Props = {
  org: OrganizationModel;
  isAuthenticated: boolean;
  isMember: boolean;
};

export const OrgJoinButton = ({
  org,
  isAuthenticated,
  isMember,
}: Props): ReactNode => {
  const revalidator = useRevalidator();
  const queryClient = useQueryClient();
  const [pendingRequest, setPendingRequest] = useState<OrganizationJoinRequestModel | null>(null);
  const [loading, setLoading] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);
  const [modalOpened, {open: openModal, close: closeModal}] = useDisclosure(false);
  const [message, setMessage] = useState("");

  const checkPendingRequest = useCallback(async () => {
    if (!isAuthenticated || isMember) {
      return;
    }
    setLoading(true);
    const [, data] = await api.getMyOrganizationJoinRequest(org.login);
    setLoading(false);
    if (data?.request) {
      setPendingRequest(data.request);
    } else {
      setPendingRequest(null);
    }
  }, [isAuthenticated, isMember, org.login]);

  useEffect(() => {
    checkPendingRequest();
  }, [checkPendingRequest]);

  if (!isAuthenticated) {
    return null;
  }

  if (isMember) {
    return <Badge color="green" variant="light" size="lg">Участник организации</Badge>;
  }

  if (org.join_policy === "invite_only" && !pendingRequest) {
    return null;
  }

  const handleJoinOpen = async () => {
    setActionLoading(true);
    const [error, data] = await api.createOrganizationJoinRequest(org.login);
    setActionLoading(false);
    if (error) {
      notifications.show({
        title: "Ошибка",
        message: error.message,
        color: "red",
      });
      return;
    }
    if (data?.joined) {
      notifications.show({
        message: `Вы успешно вступили в организацию ${org.name}!`,
        color: "green",
      });
      void queryClient.invalidateQueries({queryKey: ["org-members-permission", org.login]});
      void revalidator.revalidate();
    }
  };

  const handleRequestSubmit = async () => {
    setActionLoading(true);
    const [error, data] = await api.createOrganizationJoinRequest(org.login, {
      message: message.trim() || undefined
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

    if (data?.joined) {
      notifications.show({
        message: `Вы вступили в организацию ${org.name}!`,
        color: "green",
      });
      void queryClient.invalidateQueries({queryKey: ["org-members-permission", org.login]});
      void revalidator.revalidate();
    } else if (data?.request) {
      notifications.show({
        message: "Заявка отправлена администраторам организации",
        color: "blue",
      });
      setPendingRequest(data.request);
      dispatchNotificationsUpdated();
    }
  };

  const handleCancelRequest = async () => {
    setActionLoading(true);
    const [error] = await api.cancelOrganizationJoinRequest(org.login);
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
      message: "Заявка отменена",
      color: "blue",
    });
    setPendingRequest(null);
    dispatchNotificationsUpdated();
  };

  if (pendingRequest) {
    return (
      <Group gap="xs" align="center">
        <Badge color="orange" variant="light" size="lg">
          Заявка на рассмотрении
        </Badge>
        <Button
          size="sm"
          variant="subtle"
          color="red"
          leftSection={<IconX size={16} />}
          onClick={handleCancelRequest}
          loading={actionLoading}
        >
          Отменить заявку
        </Button>
      </Group>
    );
  }

  if (org.join_policy === "open") {
    return (
      <Button
        size="md"
        color="blue"
        leftSection={<IconUserPlus size={18} />}
        onClick={handleJoinOpen}
        loading={actionLoading}
      >
        Вступить в организацию
      </Button>
    );
  }

  // By request
  return (
    <>
      <Button
        size="md"
        color="blue"
        variant="light"
        leftSection={<IconUserPlus size={18} />}
        onClick={openModal}
        loading={loading}
      >
        Подать заявку на вступление
      </Button>

      <Modal
        opened={modalOpened}
        onClose={closeModal}
        title={`Заявка на вступление в ${org.name}`}
        centered
      >
        <Stack gap="md">
          <Text size="sm" c="dimmed">
            Администраторы организации рассмотрят вашу заявку. Вы можете оставить комментарий.
          </Text>
          <Textarea
            label="Комментарий к заявке (необязательно)"
            placeholder="Например: Ученик 10Б класса"
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
