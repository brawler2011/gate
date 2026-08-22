"use client";

import {
  Alert,
  Anchor,
  Box,
  Button,
  Center,
  Group,
  Loader,
  Paper,
  PasswordInput,
  Stack,
  Text,
  TextInput,
  ThemeIcon,
  Title,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconAlertCircle, IconCheck, IconMailCheck} from "@tabler/icons-react";
import Image from "next/image";
import Link from "next/link";
import {useSearchParams} from "next/navigation";
import {Suspense, useState, type ReactNode} from "react";

import {api} from "@/lib/api";

const RegistrationPage = (): ReactNode => {
  return (
    <Suspense
      fallback={
        <Center h="100vh">
          <Loader size="lg" />
        </Center>
      }
    >
      <RegistrationPageContent />
    </Suspense>
  );
};

const RegistrationPageContent = () => {
  const searchParams = useSearchParams();
  const returnTo = searchParams.get("return_to") || "/";

  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [isSuccess, setIsSuccess] = useState(false);
  const [resendLoading, setResendLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const [err] = await api.register({
        requestBody: {username, email, password},
      });
      if (!err) {
        setIsSuccess(true);
      } else {
        setError(err?.message || "Ошибка при регистрации");
      }
    } catch {
      setError("Не удалось подключиться к серверу");
    } finally {
      setLoading(false);
    }
  };

  const handleResend = async () => {
    setResendLoading(true);
    try {
      const [err] = await api.resendVerification({
        requestBody: {identifier: email || username},
      });
      if (!err) {
        notifications.show({
          title: "Письмо отправлено",
          message: `Ссылка для подтверждения отправлена на ${email}`,
          color: "green",
          icon: <IconCheck size={18} />,
        });
      } else {
        notifications.show({
          title: "Ошибка",
          message: err.message || "Не удалось отправить письмо",
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
    <Box
      style={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: "1rem",
      }}
    >
      <Stack
        align="center"
        gap={32}
        style={{width: "min(550px, calc(100vw - 2rem))"}}
      >
        <Link href="/" style={{textDecoration: "none", color: "inherit"}}>
          <Group justify="center" gap="md">
            <Image
              src="/gate_logo.svg"
              alt="Gate"
              width={56}
              height={56}
              priority
            />
            <Title order={1} fz={36}>
              Gate
            </Title>
          </Group>
        </Link>

        <Paper
          radius="md"
          p={32}
          withBorder
          shadow="sm"
          style={{width: "100%"}}
        >
          {isSuccess ? (
            <Stack align="center" gap="md" ta="center">
              <ThemeIcon size={64} radius="xl" color="blue" variant="light">
                <IconMailCheck size={36} />
              </ThemeIcon>

              <Title order={2} fz={22}>
                Подтвердите адрес почты
              </Title>

              <Text c="dimmed" fz={15}>
                Мы отправили письмо с ссылкой для подтверждения регистрации на адрес <b>{email}</b>.
              </Text>

              <Text c="dimmed" fz={13}>
                Пожалуйста, проверьте папку «Входящие» (и «Спам») и перейдите по ссылке в письме для завершения регистрации.
              </Text>

              <Group justify="center" mt="md" gap="sm" w="100%">
                <Button
                  variant="outline"
                  loading={resendLoading}
                  onClick={handleResend}
                  size="sm"
                >
                  Отправить письмо повторно
                </Button>

                <Button
                  component={Link}
                  href={`/auth/login${returnTo ? `?return_to=${encodeURIComponent(returnTo)}` : ""}`}
                  variant="filled"
                  size="sm"
                >
                  Перейти ко входу
                </Button>
              </Group>
            </Stack>
          ) : (
            <>
              <Title order={2} ta="center" mb={24} fz={22}>
                Регистрация аккаунта
              </Title>

              {error && (
                <Alert
                  icon={<IconAlertCircle size={18} />}
                  color="red"
                  mb={20}
                  title="Не удалось зарегистрироваться"
                  radius="md"
                >
                  {error}
                </Alert>
              )}

              <form onSubmit={handleSubmit}>
                <Stack gap={16}>
                  <TextInput
                    label="Имя пользователя"
                    placeholder="Введите имя пользователя"
                    required
                    size="md"
                    radius="md"
                    value={username}
                    onChange={(e) => setUsername(e.currentTarget.value)}
                  />

                  <TextInput
                    label="Email"
                    placeholder="Введите email"
                    type="email"
                    required
                    size="md"
                    radius="md"
                    value={email}
                    onChange={(e) => setEmail(e.currentTarget.value)}
                  />

                  <PasswordInput
                    label="Пароль"
                    placeholder="Введите пароль"
                    required
                    size="md"
                    radius="md"
                    value={password}
                    onChange={(e) => setPassword(e.currentTarget.value)}
                  />

                  <Button
                    type="submit"
                    fullWidth
                    size="md"
                    radius="md"
                    loading={loading}
                    mt={8}
                  >
                    Зарегистрироваться
                  </Button>
                </Stack>
              </form>

              <Text c="dimmed" ta="center" mt={24} fz={14}>
                Уже есть аккаунт?{" "}
                <Anchor
                  component={Link}
                  href={`/auth/login${returnTo ? `?return_to=${encodeURIComponent(returnTo)}` : ""}`}
                  fz={14}
                  fw={600}
                  underline="hover"
                >
                  Войти
                </Anchor>
              </Text>
            </>
          )}
        </Paper>
      </Stack>
    </Box>
  );
};

export default RegistrationPage;
