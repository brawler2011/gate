"use client";

import {
  Anchor,
  Badge,
  Button,
  Container,
  Divider,
  Group,
  Paper,
  PasswordInput,
  Select,
  Stack,
  Switch,
  Text,
  TextInput,
  ThemeIcon,
  Title,
} from "@mantine/core";
import {useForm} from "@mantine/form";
import {notifications} from "@mantine/notifications";
import {
  IconAlertCircle,
  IconArrowLeft,
  IconCheck,
  IconFileCode,
  IconKey,
  IconMail,
  IconShield,
  IconUser,
} from "@tabler/icons-react";
import Link from "next/link";
import {useRouter} from "next/navigation";
import {useState, type ReactNode} from "react";

import {TruncatedWithCopy} from "@/components/shared/TruncatedWithCopy";
import {api} from "@/lib/api";
import {APP_COLORS} from "@/lib/theme/colors";

import type {UpdateUserRequestModel, UserModel} from "@/contracts/core/v1";

type Props = {
  user: UserModel;
};

export const AdminUserEditContent = ({user: initialUser}: Props): ReactNode => {
  const router = useRouter();
  const [user, setUser] = useState<UserModel>(initialUser);

  // Role form
  const [role, setRole] = useState<string>(user.role);
  const [roleLoading, setRoleLoading] = useState(false);

  // Email form
  const [emailLoading, setEmailLoading] = useState(false);
  const emailForm = useForm({
    initialValues: {
      email: user.email || "",
      withConfirmation: true,
    },
    validate: {
      email: (val) => (!/^\S+@\S+\.\S+$/.test(val) ? "Некорректный email" : null),
    },
  });

  // Direct Password form
  const [setPasswordLoading, setSetPasswordLoading] = useState(false);
  const passwordForm = useForm({
    initialValues: {
      password: "",
    },
    validate: {
      password: (val) => (val.length < 8 ? "Пароль должен быть не менее 8 символов" : null),
    },
  });

  // Reset email loading
  const [resetEmailLoading, setResetEmailLoading] = useState(false);
  // Resend verification loading
  const [resendVerificationLoading, setResendVerificationLoading] = useState(false);

  const handleUpdateRole = async () => {
    if (role === user.role) {
      return;
    }
    setRoleLoading(true);
    try {
      const [error] = await api.updateUser({
        username: user.username,
        requestBody: {role: role as UpdateUserRequestModel.role},
      });

      if (error) {
        notifications.show({
          title: "Ошибка",
          message: error.message || "Не удалось изменить роль",
          color: "red",
        });
        return;
      }

      setUser((prev) => ({...prev, role}));
      notifications.show({
        title: "Успех",
        message: `Роль пользователя изменена на ${role === "admin" ? "Администратор" : "Пользователь"}`,
        color: "green",
        icon: <IconCheck size={18} />,
      });
      router.refresh();
    } catch {
      notifications.show({
        title: "Ошибка",
        message: "Не удалось подключиться к серверу",
        color: "red",
      });
    } finally {
      setRoleLoading(false);
    }
  };

  const handleChangeEmail = async (values: typeof emailForm.values) => {
    setEmailLoading(true);
    try {
      const [error] = await api.adminChangeEmail({
        username: user.username,
        requestBody: {
          email: values.email.trim(),
          with_confirmation: values.withConfirmation,
        },
      });

      if (error) {
        notifications.show({
          title: "Ошибка",
          message: error.message || "Не удалось изменить email",
          color: "red",
        });
        return;
      }

      if (!values.withConfirmation) {
        setUser((prev) => ({
          ...prev,
          email: values.email.trim(),
          is_email_verified: true,
        }));
        notifications.show({
          title: "Email обновлен",
          message: "Email успешно изменен в базе данных",
          color: "green",
          icon: <IconCheck size={18} />,
        });
      } else {
        notifications.show({
          title: "Запрос отправлен",
          message: `Письмо с подтверждением отправлено на ${values.email.trim()}`,
          color: "green",
          icon: <IconCheck size={18} />,
        });
      }
      router.refresh();
    } catch {
      notifications.show({
        title: "Ошибка",
        message: "Не удалось подключиться к серверу",
        color: "red",
      });
    } finally {
      setEmailLoading(false);
    }
  };

  const handleSetPassword = async (values: typeof passwordForm.values) => {
    setSetPasswordLoading(true);
    try {
      const [error] = await api.adminSetPassword({
        username: user.username,
        requestBody: {password: values.password},
      });

      if (error) {
        notifications.show({
          title: "Ошибка",
          message: error.message || "Не удалось установить пароль",
          color: "red",
        });
        return;
      }

      passwordForm.reset();
      notifications.show({
        title: "Пароль установлен",
        message: "Новый пароль установлен. Все активные сессии пользователя завершены.",
        color: "green",
        icon: <IconCheck size={18} />,
      });
    } catch {
      notifications.show({
        title: "Ошибка",
        message: "Не удалось подключиться к серверу",
        color: "red",
      });
    } finally {
      setSetPasswordLoading(false);
    }
  };

  const handleSendPasswordReset = async () => {
    if (!user.email) {
      notifications.show({
        title: "Ошибка",
        message: "У пользователя не указан email",
        color: "red",
      });
      return;
    }

    setResetEmailLoading(true);
    try {
      const [error] = await api.adminSendPasswordReset({
        username: user.username,
      });

      if (error) {
        notifications.show({
          title: "Ошибка",
          message: error.message || "Не удалось отправить письмо для сброса",
          color: "red",
        });
        return;
      }

      notifications.show({
        title: "Письмо отправлено",
        message: `Ссылка для сброса пароля отправлена на ${user.email}`,
        color: "green",
        icon: <IconCheck size={18} />,
      });
    } catch {
      notifications.show({
        title: "Ошибка",
        message: "Не удалось подключиться к серверу",
        color: "red",
      });
    } finally {
      setResetEmailLoading(false);
    }
  };

  const handleResendVerification = async () => {
    if (!user.email) {
      return;
    }

    setResendVerificationLoading(true);
    try {
      const [error] = await api.adminResendVerification({
        username: user.username,
      });

      if (error) {
        notifications.show({
          title: "Ошибка",
          message: error.message || "Не удалось отправить письмо",
          color: "red",
        });
        return;
      }

      notifications.show({
        title: "Письмо отправлено",
        message: `Ссылка подтверждения отправлена на ${user.email}`,
        color: "green",
        icon: <IconCheck size={18} />,
      });
    } catch {
      notifications.show({
        title: "Ошибка",
        message: "Не удалось подключиться к серверу",
        color: "red",
      });
    } finally {
      setResendVerificationLoading(false);
    }
  };

  return (
    <Container size="md" py="xl">
      <Stack gap="xl">
        <Group justify="space-between" align="center">
          <Anchor
            component={Link}
            href="/admin/users"
            c="dimmed"
            fz={14}
            style={{display: "inline-flex", alignItems: "center", gap: 6}}
          >
            <IconArrowLeft size={16} /> Назад к списку пользователей
          </Anchor>

          <Button
            component={Link}
            href={`/admin/submissions?userId=${user.id}`}
            variant="light"
            size="xs"
            leftSection={<IconFileCode size={16} />}
          >
            Посылки пользователя
          </Button>
        </Group>

        <div>
          <Group gap="sm" align="center">
            <Title order={1} fz={26}>
              @{user.username}
            </Title>
            <Anchor component={Link} href={`/@${user.username}`} fz={13} c="dimmed">
              (открыть профиль)
            </Anchor>
          </Group>
          <Text c="dimmed" fz={14}>
            Редактирование профиля и параметров безопасности пользователя
          </Text>
        </div>

        {/* 1. Overview */}
        <Paper withBorder radius="md" p="xl">
          <Stack gap="md">
            <Group justify="space-between" align="center">
              <Group gap="sm">
                <ThemeIcon size={40} radius="md" color={APP_COLORS.users} variant="light">
                  <IconUser size={24} />
                </ThemeIcon>
                <div>
                  <Title order={3} fz={18}>
                    Данные пользователя
                  </Title>
                  <Text c="dimmed" fz={13}>
                    Системная информация
                  </Text>
                </div>
              </Group>
              <Badge
                color={user.role === "admin" ? APP_COLORS.admin : "blue"}
                variant="light"
                size="md"
              >
                {user.role === "admin" ? "Администратор" : "Пользователь"}
              </Badge>
            </Group>

            <Divider />

            <Stack gap="xs">
              <Group justify="space-between">
                <Text fw={500} fz={14}>ID пользователя:</Text>
                <TruncatedWithCopy value={user.id} />
              </Group>

              <Group justify="space-between">
                <Text fw={500} fz={14}>Email:</Text>
                <Group gap="xs">
                  <Text fz={14}>{user.email || "Не указан"}</Text>
                  {user.email && (
                    user.is_email_verified ? (
                      <Badge color="green" variant="light" size="sm">
                        Подтверждён
                      </Badge>
                    ) : (
                      <Badge color="orange" variant="light" size="sm">
                        Не подтверждён
                      </Badge>
                    )
                  )}
                </Group>
              </Group>

              <Group justify="space-between">
                <Text fw={500} fz={14}>Дата регистрации:</Text>
                <Text fz={14} c="dimmed">
                  {new Date(user.createdAt).toLocaleDateString("ru-RU", {
                    year: "numeric",
                    month: "long",
                    day: "numeric",
                    hour: "2-digit",
                    minute: "2-digit",
                  })}
                </Text>
              </Group>
            </Stack>
          </Stack>
        </Paper>

        {/* 2. Role Management */}
        <Paper withBorder radius="md" p="xl">
          <Stack gap="md">
            <Group gap="sm">
              <ThemeIcon size={40} radius="md" color={APP_COLORS.admin} variant="light">
                <IconShield size={24} />
              </ThemeIcon>
              <div>
                <Title order={3} fz={18}>
                  Роль пользователя
                </Title>
                <Text c="dimmed" fz={13}>
                  Назначение прав администратора или обычного пользователя
                </Text>
              </div>
            </Group>

            <Divider />

            <Group align="flex-end">
              <Select
                label="Роль"
                value={role}
                onChange={(val) => val && setRole(val)}
                data={[
                  {value: "user", label: "Пользователь"},
                  {value: "admin", label: "Администратор"},
                ]}
                style={{flex: 1, maxWidth: 300}}
              />
              <Button
                onClick={handleUpdateRole}
                loading={roleLoading}
                disabled={role === user.role}
              >
                Сохранить роль
              </Button>
            </Group>
          </Stack>
        </Paper>

        {/* 3. Email Management */}
        <Paper withBorder radius="md" p="xl">
          <Stack gap="md">
            <Group gap="sm">
              <ThemeIcon size={40} radius="md" color="teal" variant="light">
                <IconMail size={24} />
              </ThemeIcon>
              <div>
                <Title order={3} fz={18}>
                  Смена адреса электронной почты
                </Title>
                <Text c="dimmed" fz={13}>
                  Администратор может изменить email с подтверждением или напрямую
                </Text>
              </div>
            </Group>

            <Divider />

            <form onSubmit={emailForm.onSubmit(handleChangeEmail)}>
              <Stack gap="md">
                <TextInput
                  label="Новый Email"
                  placeholder="user@example.com"
                  type="email"
                  required
                  {...emailForm.getInputProps("email")}
                />

                <Switch
                  label="С подтверждением по почте (рекомендуется)"
                  description="При включении пользователю отправляется ссылка подтверждения на новый ящик, а на старый — предупреждение. При выключении email меняется в БД мгновенно."
                  checked={emailForm.values.withConfirmation}
                  onChange={(e) => emailForm.setFieldValue("withConfirmation", e.currentTarget.checked)}
                />

                <Group justify="flex-end">
                  <Button type="submit" loading={emailLoading} color="teal">
                    {emailForm.values.withConfirmation ? "Отправить подтверждение" : "Изменить email напрямую"}
                  </Button>
                </Group>
              </Stack>
            </form>
          </Stack>
        </Paper>

        {/* 4. Password Management */}
        <Paper withBorder radius="md" p="xl">
          <Stack gap="md">
            <Group gap="sm">
              <ThemeIcon size={40} radius="md" color="indigo" variant="light">
                <IconKey size={24} />
              </ThemeIcon>
              <div>
                <Title order={3} fz={18}>
                  Управление паролем
                </Title>
                <Text c="dimmed" fz={13}>
                  Сброс пароля через email или прямая установка нового пароля
                </Text>
              </div>
            </Group>

            <Divider />

            <Stack gap="md">
              <Group justify="space-between" align="center">
                <div>
                  <Text fw={500} fz={14}>Сброс пароля через email</Text>
                  <Text c="dimmed" fz={13}>
                    Отправляет пользователю стандартное письмо со ссылкой на сброс пароля
                  </Text>
                </div>
                <Button
                  variant="outline"
                  color="indigo"
                  loading={resetEmailLoading}
                  onClick={handleSendPasswordReset}
                  disabled={!user.email}
                >
                  Отправить ссылку на сброс
                </Button>
              </Group>

              <Divider label="или установить пароль вручную" labelPosition="center" />

              <form onSubmit={passwordForm.onSubmit(handleSetPassword)}>
                <Stack gap="md">
                  <PasswordInput
                    label="Новый пароль для пользователя"
                    placeholder="Введите новый пароль (не менее 8 символов)"
                    required
                    {...passwordForm.getInputProps("password")}
                  />

                  <Text c="dimmed" fz={12}>
                    Установка пароля администратором мгновенно завершит все активные сессии пользователя в системе.
                  </Text>

                  <Group justify="flex-end">
                    <Button type="submit" color="indigo" loading={setPasswordLoading}>
                      Установить пароль напрямую
                    </Button>
                  </Group>
                </Stack>
              </form>
            </Stack>
          </Stack>
        </Paper>

        {/* 5. Verification Management (if unverified) */}
        {!user.is_email_verified && user.email && (
          <Paper withBorder radius="md" p="xl" style={{borderColor: "var(--mantine-color-orange-5)"}}>
            <Stack gap="md">
              <Group justify="space-between" align="center">
                <div>
                  <Group gap="xs">
                    <ThemeIcon size={28} radius="xl" color="orange" variant="light">
                      <IconAlertCircle size={18} />
                    </ThemeIcon>
                    <Text fw={600} fz={16}>Почта не подтверждена</Text>
                  </Group>
                  <Text c="dimmed" fz={13} mt={4}>
                    Пользователь еще не активировал свою учетную запись по ссылке из письма
                  </Text>
                </div>

                <Button
                  color="orange"
                  variant="light"
                  loading={resendVerificationLoading}
                  onClick={handleResendVerification}
                >
                  Отправить ссылку подтверждения повторно
                </Button>
              </Group>
            </Stack>
          </Paper>
        )}
      </Stack>
    </Container>
  );
};
