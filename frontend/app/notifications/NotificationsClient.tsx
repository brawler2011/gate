"use client";

import {
  ActionIcon,
  Badge,
  Button,
  Card,
  Center,
  Container,
  Group,
  Loader,
  Pagination,
  SegmentedControl,
  Stack,
  Text,
  Title,
  Tooltip,
} from "@mantine/core";
import {notifications as mantineNotifications} from "@mantine/notifications";
import {
  IconBell,
  IconBuilding,
  IconCheck,
  IconChecks,
  IconExternalLink,
  IconTrophy,
  IconX,
} from "@tabler/icons-react";
import Link from "next/link";
import {useCallback, useEffect, useState, type ReactNode} from "react";

import {ApproveOrganizationJoinRequestModel} from "@/contracts/core/v1";
import {dispatchNotificationsUpdated} from "@/hooks/useUnreadNotificationsCount";
import {api} from "@/lib/api";
import {APP_COLORS} from "@/lib/theme/colors";

import type {NotificationModel, PaginationModel} from "@/contracts/core/v1";

const formatRelativeTime = (dateStr: string): string => {
  const date = new Date(dateStr);
  const now = new Date();
  const diffSec = Math.floor((now.getTime() - date.getTime()) / 1000);

  if (diffSec < 60) {
    return "только что";
  }
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) {
    return `${diffMin} мин. назад`;
  }
  const diffHours = Math.floor(diffMin / 60);
  if (diffHours < 24) {
    return `${diffHours} ч. назад`;
  }
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays < 30) {
    return `${diffDays} дн. назад`;
  }
  return date.toLocaleDateString("ru-RU", {day: "numeric", month: "short"});
};

export const NotificationsClient = (): ReactNode => {
  const [tab, setTab] = useState<"all" | "unread">("all");
  const [page, setPage] = useState<number>(1);
  const [loading, setLoading] = useState<boolean>(true);
  const [items, setItems] = useState<NotificationModel[]>([]);
  const [pagination, setPagination] = useState<PaginationModel>({page: 1, total: 1});
  const [markingAll, setMarkingAll] = useState<boolean>(false);
  const [actionLoadingId, setActionLoadingId] = useState<string | null>(null);

  const fetchNotifications = useCallback(async () => {
    setLoading(true);
    const [error, data] = await api.listNotifications({
      page,
      pageSize: 20,
      unreadOnly: tab === "unread",
    });
    setLoading(false);
    if (!error && data) {
      setItems(data.notifications);
      setPagination(data.pagination);
    }
  }, [page, tab]);

  useEffect(() => {
    fetchNotifications();
  }, [fetchNotifications]);

  const handleMarkAsRead = async (id: string) => {
    setActionLoadingId(id);
    const [error] = await api.markNotificationAsRead({id});
    setActionLoadingId(null);
    if (!error) {
      setItems((prev) =>
        prev.map((n) => (n.id === id ? {...n, is_read: true} : n)),
      );
      dispatchNotificationsUpdated();
    }
  };

  const handleMarkAllAsRead = async () => {
    setMarkingAll(true);
    const [error] = await api.markAllNotificationsAsRead();
    setMarkingAll(false);
    if (!error) {
      setItems((prev) => prev.map((n) => ({...n, is_read: true})));
      dispatchNotificationsUpdated();
      mantineNotifications.show({
        color: "green",
        message: "Все уведомления отмечены как прочитанные",
      });
    }
  };

  // Org Invitation Actions
  const handleAcceptOrgInvitation = async (notif: NotificationModel) => {
    const data = notif.data as Record<string, unknown> | undefined;
    const invitationId = data?.invitation_id as string | undefined;
    if (!invitationId) {
      return;
    }

    setActionLoadingId(notif.id);
    const [error] = await api.acceptOrganizationInvitation({id: invitationId});
    setActionLoadingId(null);

    if (error) {
      mantineNotifications.show({
        color: "red",
        message: error.message || "Не удалось принять приглашение",
      });
      return;
    }

    mantineNotifications.show({
      color: "green",
      message: `Вы вступили в организацию ${data?.organization_name || ""}`,
    });
    await handleMarkAsRead(notif.id);
    fetchNotifications();
  };

  const handleDeclineOrgInvitation = async (notif: NotificationModel) => {
    const data = notif.data as Record<string, unknown> | undefined;
    const invitationId = data?.invitation_id as string | undefined;
    if (!invitationId) {
      return;
    }

    setActionLoadingId(notif.id);
    const [error] = await api.declineOrganizationInvitation({id: invitationId});
    setActionLoadingId(null);

    if (error) {
      mantineNotifications.show({
        color: "red",
        message: error.message || "Не удалось отклонить приглашение",
      });
      return;
    }

    mantineNotifications.show({
      color: "blue",
      message: "Приглашение отклонено",
    });
    await handleMarkAsRead(notif.id);
    fetchNotifications();
  };

  // Org Join Request Actions
  const handleApproveOrgJoinRequest = async (notif: NotificationModel) => {
    const data = notif.data as Record<string, unknown> | undefined;
    const requestId = data?.request_id as string | undefined;
    const orgLogin = data?.organization_login as string | undefined;
    if (!requestId || !orgLogin) {
      return;
    }

    setActionLoadingId(notif.id);
    const [error] = await api.approveOrganizationJoinRequest({
      login: orgLogin,
      id: requestId,
      requestBody: {role: ApproveOrganizationJoinRequestModel.role.MEMBER},
    });
    setActionLoadingId(null);

    if (error) {
      mantineNotifications.show({
        color: "red",
        message: error.message || "Не удалось одобрить заявку",
      });
      return;
    }

    mantineNotifications.show({
      color: "green",
      message: "Заявка одобрена",
    });
    await handleMarkAsRead(notif.id);
    fetchNotifications();
  };

  const handleRejectOrgJoinRequest = async (notif: NotificationModel) => {
    const data = notif.data as Record<string, unknown> | undefined;
    const requestId = data?.request_id as string | undefined;
    const orgLogin = data?.organization_login as string | undefined;
    if (!requestId || !orgLogin) {
      return;
    }

    setActionLoadingId(notif.id);
    const [error] = await api.rejectOrganizationJoinRequest({
      login: orgLogin,
      id: requestId,
    });
    setActionLoadingId(null);

    if (error) {
      mantineNotifications.show({
        color: "red",
        message: error.message || "Не удалось отклонить заявку",
      });
      return;
    }

    mantineNotifications.show({
      color: "blue",
      message: "Заявка отклонена",
    });
    await handleMarkAsRead(notif.id);
    fetchNotifications();
  };

  // Contest Join Request Actions
  const handleApproveContestJoinRequest = async (notif: NotificationModel) => {
    const data = notif.data as Record<string, unknown> | undefined;
    const requestId = data?.request_id as string | undefined;
    const orgLogin = data?.organization_login as string | undefined;
    const contestLogin = data?.contest_login as string | undefined;
    if (!requestId || !orgLogin || !contestLogin) {
      return;
    }

    setActionLoadingId(notif.id);
    const [error] = await api.approveContestJoinRequest({
      orgLogin,
      contestLogin,
      id: requestId,
    });
    setActionLoadingId(null);

    if (error) {
      mantineNotifications.show({
        color: "red",
        message: error.message || "Не удалось одобрить заявку",
      });
      return;
    }

    mantineNotifications.show({
      color: "green",
      message: "Заявка на участие одобрена",
    });
    await handleMarkAsRead(notif.id);
    fetchNotifications();
  };

  const handleRejectContestJoinRequest = async (notif: NotificationModel) => {
    const data = notif.data as Record<string, unknown> | undefined;
    const requestId = data?.request_id as string | undefined;
    const orgLogin = data?.organization_login as string | undefined;
    const contestLogin = data?.contest_login as string | undefined;
    if (!requestId || !orgLogin || !contestLogin) {
      return;
    }

    setActionLoadingId(notif.id);
    const [error] = await api.rejectContestJoinRequest({
      orgLogin,
      contestLogin,
      id: requestId,
    });
    setActionLoadingId(null);

    if (error) {
      mantineNotifications.show({
        color: "red",
        message: error.message || "Не удалось отклонить заявку",
      });
      return;
    }

    mantineNotifications.show({
      color: "blue",
      message: "Заявка отклонена",
    });
    await handleMarkAsRead(notif.id);
    fetchNotifications();
  };

  const getNotificationIcon = (type: string) => {
    if (type.startsWith("org_")) {
      return <IconBuilding size={20} color="var(--mantine-color-blue-6)" />;
    }
    if (type.startsWith("contest_")) {
      return <IconTrophy size={20} color="var(--mantine-color-orange-6)" />;
    }
    return <IconBell size={20} color="var(--mantine-color-gray-6)" />;
  };

  const emptyMessage = tab === "unread"
    ? "Нет новых непрочитанных уведомлений"
    : "У вас пока нет уведомлений";

  return (
    <Container size="md" py="xl">
      <Stack gap="lg">
        <Group justify="space-between" align="center" wrap="wrap">
          <Group gap="xs">
            <Title order={2}>Уведомления</Title>
          </Group>
          <Group gap="sm">
            <SegmentedControl
              value={tab}
              onChange={(val) => {
                setTab(val as "all" | "unread");
                setPage(1);
              }}
              data={[
                {label: "Все", value: "all"},
                {label: "Непрочитанные", value: "unread"},
              ]}
              size="sm"
            />
            <Button
              variant="light"
              color="gray"
              size="sm"
              leftSection={<IconChecks size={16} />}
              onClick={handleMarkAllAsRead}
              loading={markingAll}
            >
              Прочитать все
            </Button>
          </Group>
        </Group>

        {loading && (
          <Center py="xl">
            <Loader size="md" />
          </Center>
        )}

        {!loading && items.length === 0 && (
          <Center py={60}>
            <Stack align="center" gap="xs">
              <IconBell size={48} stroke={1.5} color="var(--mantine-color-dimmed)" />
              <Text c="dimmed" size="lg">
                {emptyMessage}
              </Text>
            </Stack>
          </Center>
        )}

        {!loading && items.length > 0 && (
          <Stack gap="sm">
            {items.map((notif) => {
              const data = notif.data as Record<string, unknown> | undefined;
              const hasOrgInviteAction =
                notif.type === "org_invitation" && Boolean(data?.invitation_id);
              const hasOrgRequestAction =
                notif.type === "org_join_request" && Boolean(data?.request_id) && Boolean(data?.organization_login);
              const hasContestRequestAction =
                notif.type === "contest_join_request" &&
                Boolean(data?.request_id) &&
                Boolean(data?.organization_login) &&
                Boolean(data?.contest_login);

              const isActing = actionLoadingId === notif.id;

              return (
                <Card
                  key={notif.id}
                  withBorder
                  padding="md"
                  radius="md"
                  style={{
                    backgroundColor: notif.is_read
                      ? undefined
                      : "var(--mantine-color-blue-light)",
                    borderColor: notif.is_read
                      ? undefined
                      : "var(--mantine-color-blue-light-color)",
                    transition: "all 0.15s ease",
                  }}
                >
                  <Group justify="space-between" align="flex-start" wrap="nowrap" gap="md">
                    <Group align="flex-start" gap="md" style={{flex: 1, minWidth: 0}}>
                      <Center
                        w={36}
                        h={36}
                        style={{
                          borderRadius: "50%",
                          backgroundColor: "var(--mantine-color-default)",
                          flexShrink: 0,
                        }}
                      >
                        {getNotificationIcon(notif.type)}
                      </Center>
                      <Stack gap={4} style={{flex: 1, minWidth: 0}}>
                        <Group gap="xs" align="center" wrap="wrap">
                          <Text fw={600} size="sm">
                            {notif.title}
                          </Text>
                          {!notif.is_read && (
                            <Badge size="xs" color="blue" variant="filled">
                              Новое
                            </Badge>
                          )}
                          <Text size="xs" c="dimmed">
                            {formatRelativeTime(notif.created_at)}
                          </Text>
                        </Group>
                        <Text size="sm" c="dimmed" style={{wordBreak: "break-word"}}>
                          {notif.body}
                        </Text>

                        {/* Interactive Buttons */}
                        <Group gap="xs" mt="xs" wrap="wrap">
                          {hasOrgInviteAction && (
                            <>
                              <Button
                                size="xs"
                                color="green"
                                variant="filled"
                                leftSection={<IconCheck size={14} />}
                                onClick={() => handleAcceptOrgInvitation(notif)}
                                loading={isActing}
                              >
                                Принять
                              </Button>
                              <Button
                                size="xs"
                                color="red"
                                variant="light"
                                leftSection={<IconX size={14} />}
                                onClick={() => handleDeclineOrgInvitation(notif)}
                                loading={isActing}
                              >
                                Отклонить
                              </Button>
                            </>
                          )}

                          {hasOrgRequestAction && (
                            <>
                              <Button
                                size="xs"
                                color="green"
                                variant="filled"
                                leftSection={<IconCheck size={14} />}
                                onClick={() => handleApproveOrgJoinRequest(notif)}
                                loading={isActing}
                              >
                                Одобрить
                              </Button>
                              <Button
                                size="xs"
                                color="red"
                                variant="light"
                                leftSection={<IconX size={14} />}
                                onClick={() => handleRejectOrgJoinRequest(notif)}
                                loading={isActing}
                              >
                                Отклонить
                              </Button>
                            </>
                          )}

                          {hasContestRequestAction && (
                            <>
                              <Button
                                size="xs"
                                color="green"
                                variant="filled"
                                leftSection={<IconCheck size={14} />}
                                onClick={() => handleApproveContestJoinRequest(notif)}
                                loading={isActing}
                              >
                                Одобрить
                              </Button>
                              <Button
                                size="xs"
                                color="red"
                                variant="light"
                                leftSection={<IconX size={14} />}
                                onClick={() => handleRejectContestJoinRequest(notif)}
                                loading={isActing}
                              >
                                Отклонить
                              </Button>
                            </>
                          )}

                          {notif.link && (
                            <Button
                              component={Link}
                              href={notif.link}
                              size="xs"
                              variant="subtle"
                              color="blue"
                              rightSection={<IconExternalLink size={14} />}
                              onClick={() => {
                                if (!notif.is_read) {
                                  handleMarkAsRead(notif.id);
                                }
                              }}
                            >
                              Перейти
                            </Button>
                          )}
                        </Group>
                      </Stack>
                    </Group>

                    {!notif.is_read && (
                      <Tooltip label="Отметить как прочитанное">
                        <ActionIcon
                          variant="subtle"
                          color="gray"
                          size="sm"
                          onClick={() => handleMarkAsRead(notif.id)}
                          loading={isActing}
                        >
                          <IconCheck size={16} />
                        </ActionIcon>
                      </Tooltip>
                    )}
                  </Group>
                </Card>
              );
            })}
          </Stack>
        )}

        {pagination.total > 1 && (
          <Center mt="md">
            <Pagination
              value={page}
              onChange={setPage}
              total={pagination.total}
              color={APP_COLORS.actions.primary}
            />
          </Center>
        )}
      </Stack>
    </Container>
  );
};
