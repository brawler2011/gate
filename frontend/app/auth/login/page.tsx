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
  Title,
} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconAlertCircle, IconCheck} from "@tabler/icons-react";
import Image from "next/image";
import Link from "next/link";
import {useRouter, useSearchParams} from "next/navigation";
import {Suspense, useState, type ReactNode} from "react";

import {useSession} from "@/contexts/SessionContext";
import {api} from "@/lib/api";

const LoginPage = (): ReactNode => {
  return (
    <Suspense
      fallback={
        <Center h="100vh">
          <Loader size="lg" />
        </Center>
      }
    >
      <LoginPageContent />
    </Suspense>
  );
};

const LoginPageContent = () => {
  const router = useRouter();
  const searchParams = useSearchParams();
  const returnTo = searchParams.get("return_to") || "/";
  const {setUser} = useSession();

  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [isUnverified, setIsUnverified] = useState(false);
  const [resendLoading, setResendLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setIsUnverified(false);
    setLoading(true);

    try {
      const [err, data] = await api.login({
        requestBody: {identifier, password},
      });
      if (!err && data) {
        setUser(data.user);
        router.push(returnTo);
        router.refresh();
      } else {
        const msg = err?.message || "Неверный логин или пароль";
        if (msg.toLowerCase().includes("email not verified") || msg.toLowerCase().includes("не подтверждена")) {
          setIsUnverified(true);
          setError("Почта для этого аккаунта ещё не подтверждена. Пожалуйста, проверьте ваш почтовый ящик.");
        } else {
          setError(msg);
        }
      }
    } catch {
      setError("Не удалось подключиться к серверу");
    } finally {
      setLoading(false);
    }
  };

  const handleResend = async () => {
    if (!identifier) {
      return;
    }
    setResendLoading(true);
    try {
      const [err] = await api.resendVerification({
        requestBody: {identifier},
      });
      if (!err) {
        notifications.show({
          title: "Письмо отправлено",
          message: "Ссылка для подтверждения отправлена на ваш email",
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
          <Title order={2} ta="center" mb={24} fz={22}>
            Вход в аккаунт
          </Title>

          {error && (
            <Alert
              icon={<IconAlertCircle size={18} />}
              color="red"
              mb={20}
              title="Не удалось войти"
              radius="md"
            >
              <Text fz={14}>{error}</Text>
              {isUnverified && (
                <Button
                  size="xs"
                  variant="white"
                  color="red"
                  mt="xs"
                  loading={resendLoading}
                  onClick={handleResend}
                >
                  Отправить письмо для подтверждения повторно
                </Button>
              )}
            </Alert>
          )}

          <form onSubmit={handleSubmit}>
            <Stack gap={16}>
              <TextInput
                label="Email или имя пользователя"
                placeholder="Введите email или имя пользователя"
                required
                size="md"
                radius="md"
                value={identifier}
                onChange={(e) => setIdentifier(e.currentTarget.value)}
              />

              <div>
                <PasswordInput
                  label="Пароль"
                  placeholder="Введите пароль"
                  required
                  size="md"
                  radius="md"
                  value={password}
                  onChange={(e) => setPassword(e.currentTarget.value)}
                />
                <Group justify="flex-end" mt={6}>
                  <Anchor
                    component={Link}
                    href="/auth/forgot-password"
                    fz={13}
                    c="dimmed"
                    underline="hover"
                  >
                    Забыли пароль?
                  </Anchor>
                </Group>
              </div>

              <Button
                type="submit"
                fullWidth
                size="md"
                radius="md"
                loading={loading}
                mt={8}
              >
                Войти
              </Button>
            </Stack>
          </form>

          <Text c="dimmed" ta="center" mt={24} fz={14}>
            Ещё нет аккаунта?{" "}
            <Anchor
              component={Link}
              href={`/auth/registration${returnTo ? `?return_to=${encodeURIComponent(returnTo)}` : ""}`}
              fz={14}
              fw={600}
              underline="hover"
            >
              Зарегистрироваться
            </Anchor>
          </Text>
        </Paper>
      </Stack>
    </Box>
  );
};

export default LoginPage;
