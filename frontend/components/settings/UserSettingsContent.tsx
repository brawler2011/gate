"use client";

import {
  Alert,
  Badge,
  Button,
  Container,
  Divider,
  Group,
  Paper,
  PasswordInput,
  Stack,
  Text,
  TextInput,
  ThemeIcon,
  Title,
} from "@mantine/core";
import {useForm} from "@mantine/form";
import {notifications} from "@mantine/notifications";
import {
  IconCheck,
  IconKey,
  IconMail,
  IconUser,
} from "@tabler/icons-react";
import {useState, type ReactNode} from "react";

import {ClaimTemporaryAccountSection} from "@/components/users/ClaimTemporaryAccountSection";
import {useSession} from "@/contexts/SessionContext";
import {api} from "@/lib/api";
import {APP_COLORS} from "@/lib/theme/colors";

import type {UserModel} from "@/contracts/core/v1";

type Props = {
  initialUser: UserModel;
};

export const UserSettingsContent = ({initialUser}: Props): ReactNode => {
  const {user: sessionUser, setUser} = useSession();
  const user = sessionUser || initialUser;

  // Change Password State
  const [passwordLoading, setPasswordLoading] = useState(false);
  const passwordForm = useForm({
    initialValues: {
      oldPassword: "",
      newPassword: "",
      confirmNewPassword: "",
    },
    validate: {
      oldPassword: (value) => (!value ? "Введите текущий пароль" : null),
      newPassword: (value) =>
        value.length < 8 ? "Пароль должен содержать не менее 8 символов" : null,
      confirmNewPassword: (value, values) =>
        value !== values.newPassword ? "Пароли не совпадают" : null,
    },
  });

  // Change Email State
  const [emailLoading, setEmailLoading] = useState(false);
  const [emailChangeSuccess, setEmailChangeSuccess] = useState<string | null>(null);
  const [resendLoading, setResendLoading] = useState(false);

  const emailForm = useForm({
    initialValues: {
      currentPassword: "",
      newEmail: "",
    },
    validate: {
      currentPassword: (value) => (!value ? "Введите текущий пароль" : null),
      newEmail: (value) =>
        !/^\S+@\S+\.\S+$/.test(value) ? "Некорректный email" : null,
    },
  });

  const handleChangePassword = async (values: typeof passwordForm.values) => {
    setPasswordLoading(true);
    try {
      const [error, data] = await api.changePassword({
        requestBody: {
          old_password: values.oldPassword,
          new_password: values.newPassword,
        },
      });

      if (error) {
        notifications.show({
          title: "Ошибка смены пароля",
          message: error.message || "Неверный текущий пароль",
          color: "red",
        });
        return;
      }

      if (data?.user) {
        setUser(data.user);
      }

      notifications.show({
        title: "Пароль изменён",
        message: "Пароль успешно обновлён. Все остальные сессии завершены.",
        color: "green",
        icon: <IconCheck size={18} />,
      });
      passwordForm.reset();
    } catch {
      notifications.show({
        title: "Ошибка",
        message: "Не удалось подключиться к серверу",
        color: "red",
      });
    } finally {
      setPasswordLoading(false);
    }
  };

  const handleRequestEmailChange = async (values: typeof emailForm.values) => {
    setEmailLoading(true);
    setEmailChangeSuccess(null);
    try {
      const [error] = await api.requestEmailChange({
        requestBody: {
          password: values.currentPassword,
          new_email: values.newEmail.trim(),
        },
      });

      if (error) {
        notifications.show({
          title: "Ошибка смены email",
          message: error.message || "Не удалось запросить смену почты",
          color: "red",
        });
        return;
      }

      setEmailChangeSuccess(values.newEmail.trim());
      emailForm.reset();
      notifications.show({
        title: "Письмо отправлено",
        message: `Ссылка для подтверждения отправлена на ${values.newEmail.trim()}`,
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
      setEmailLoading(false);
    }
  };

  const handleResendCurrentEmailVerification = async () => {
    if (!user.email) {
      return;
    }
    setResendLoading(true);
    try {
      const [error] = await api.resendVerification({
        requestBody: {identifier: user.email},
      });
      if (!error) {
        notifications.show({
          title: "Письмо отправлено",
          message: `Ссылка для подтверждения отправлена на ${user.email}`,
          color: "green",
          icon: <IconCheck size={18} />,
        });
      } else {
        notifications.show({
          title: "Ошибка",
          message: error.message || "Не удалось отправить письмо",
          color: "red",
        });
      }
    } catch {
      notifications.show({
        title: "Ошибка",
        message: "Не удалось подключиться к серверу",
        color: "red",
      });
    } finally {
      setResendLoading(false);
    }
  };

  return (
    <Container size="md" py="xl">
      <Stack gap="xl">
        <div>
          <Title order={1} fz={28}>
            Настройки профиля
          </Title>
          <Text c="dimmed" fz={14}>
            Управление параметрами аккаунта, безопасностью и адресом электронной почты
          </Text>
        </div>

        {/* 1. Account Info Overview */}
        <Paper withBorder radius="md" p="xl">
          <Stack gap="md">
            <Group justify="space-between" align="center">
              <Group gap="sm">
                <ThemeIcon size={40} radius="md" color={APP_COLORS.users} variant="light">
                  <IconUser size={24} />
                </ThemeIcon>
                <div>
                  <Title order={3} fz={18}>
                    Учётная запись
                  </Title>
                  <Text c="dimmed" fz={13}>
                    Основные данные профиля
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
                <Text fw={500} fz={14}>
                  Имя пользователя:
                </Text>
                <Text fz={14}>@{user.username}</Text>
              </Group>

              <Group justify="space-between" align="center">
                <Text fw={500} fz={14}>
                  Email:
                </Text>
                <Group gap="xs">
                  <Text fz={14}>{user.email || "Не указан"}</Text>
                  {user.email && (
                    user.is_email_verified ? (
                      <Badge color="green" variant="light" size="sm">
                        Подтверждён
                      </Badge>
                    ) : (
                      <Group gap={6}>
                        <Badge color="orange" variant="light" size="sm">
                          Не подтверждён
                        </Badge>
                        <Button
                          size="compact-xs"
                          variant="subtle"
                          loading={resendLoading}
                          onClick={handleResendCurrentEmailVerification}
                        >
                          Подтвердить
                        </Button>
                      </Group>
                    )
                  )}
                </Group>
              </Group>

              <Group justify="space-between">
                <Text fw={500} fz={14}>
                  Дата регистрации:
                </Text>
                <Text fz={14} c="dimmed">
                  {new Date(user.createdAt).toLocaleDateString("ru-RU", {
                    year: "numeric",
                    month: "long",
                    day: "numeric",
                  })}
                </Text>
              </Group>
            </Stack>
          </Stack>
        </Paper>

        {/* 2. Change Email */}
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
                  Для смены email необходимо ввести текущий пароль
                </Text>
              </div>
            </Group>

            <Divider />

            {emailChangeSuccess && (
              <Alert
                icon={<IconCheck size={18} />}
                color="green"
                title="Запрос на смену email отправлен"
                radius="md"
              >
                Мы отправили ссылку для подтверждения на адрес <b>{emailChangeSuccess}</b>,
                а также оповещение на ваш текущий email. Перейдите по ссылке для завершения смены.
              </Alert>
            )}

            <form onSubmit={emailForm.onSubmit(handleRequestEmailChange)}>
              <Stack gap="md">
                <TextInput
                  label="Новый Email"
                  placeholder="new-email@example.com"
                  type="email"
                  required
                  {...emailForm.getInputProps("newEmail")}
                />

                <PasswordInput
                  label="Текущий пароль"
                  placeholder="Введите пароль для подтверждения"
                  required
                  {...emailForm.getInputProps("currentPassword")}
                />

                <Text c="dimmed" fz={12}>
                  На новый адрес будет отправлена ссылка для подтверждения.
                  До момента перехода по ссылке старый адрес останется активным.
                </Text>

                <Group justify="flex-end">
                  <Button type="submit" loading={emailLoading} color="teal">
                    Запросить смену email
                  </Button>
                </Group>
              </Stack>
            </form>
          </Stack>
        </Paper>

        {/* 3. Change Password */}
        <Paper withBorder radius="md" p="xl">
          <Stack gap="md">
            <Group gap="sm">
              <ThemeIcon size={40} radius="md" color="indigo" variant="light">
                <IconKey size={24} />
              </ThemeIcon>
              <div>
                <Title order={3} fz={18}>
                  Смена пароля
                </Title>
                <Text c="dimmed" fz={13}>
                  Обновите пароль вашей учётной записи
                </Text>
              </div>
            </Group>

            <Divider />

            <form onSubmit={passwordForm.onSubmit(handleChangePassword)}>
              <Stack gap="md">
                <PasswordInput
                  label="Текущий пароль"
                  placeholder="Введите текущий пароль"
                  required
                  {...passwordForm.getInputProps("oldPassword")}
                />

                <PasswordInput
                  label="Новый пароль"
                  placeholder="Не менее 8 символов"
                  required
                  {...passwordForm.getInputProps("newPassword")}
                />

                <PasswordInput
                  label="Подтверждение нового пароля"
                  placeholder="Повторите новый пароль"
                  required
                  {...passwordForm.getInputProps("confirmNewPassword")}
                />

                <Text c="dimmed" fz={12}>
                  После смены пароля все остальные активные сессии на других устройствах будут принудительно завершены.
                </Text>

                <Group justify="flex-end">
                  <Button type="submit" loading={passwordLoading} color="indigo">
                    Изменить пароль
                  </Button>
                </Group>
              </Stack>
            </form>
          </Stack>
        </Paper>

        {/* 4. Claim Temporary Account Section */}
        <ClaimTemporaryAccountSection />
      </Stack>
    </Container>
  );
};
