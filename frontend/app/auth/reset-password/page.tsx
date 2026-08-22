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
  ThemeIcon,
  Title,
} from "@mantine/core";
import {IconAlertCircle, IconCheck} from "@tabler/icons-react";
import Image from "next/image";
import Link from "next/link";
import {useSearchParams} from "next/navigation";
import {Suspense, useState, type ReactNode} from "react";

import {api} from "@/lib/api";

const ResetPasswordPage = (): ReactNode => {
  return (
    <Suspense
      fallback={
        <Center h="100vh">
          <Loader size="lg" />
        </Center>
      }
    >
      <ResetPasswordContent />
    </Suspense>
  );
};

const ResetPasswordContent = () => {
  const searchParams = useSearchParams();
  const token = searchParams.get("token") || "";

  const [password, setPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (!token) {
      setError("Токен сброса пароля отсутствует или недействителен.");
      return;
    }

    if (password.length < 8) {
      setError("Пароль должен содержать не менее 8 символов.");
      return;
    }

    if (password !== confirmPassword) {
      setError("Пароли не совпадают.");
      return;
    }

    setLoading(true);

    try {
      const [err] = await api.resetPassword({
        requestBody: {token, password},
      });
      if (!err) {
        setSuccess(true);
      } else {
        setError(err?.message || "Не удалось сбросить пароль. Возможно, срок действия ссылки истёк.");
      }
    } catch {
      setError("Не удалось подключиться к серверу");
    } finally {
      setLoading(false);
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
          {success ? (
            <Stack align="center" gap="md" ta="center">
              <ThemeIcon size={64} radius="xl" color="green" variant="light">
                <IconCheck size={36} />
              </ThemeIcon>

              <Title order={2} fz={22}>
                Пароль успешно изменён!
              </Title>

              <Text c="dimmed" fz={15}>
                Теперь вы можете войти в свой аккаунт, используя новый пароль.
              </Text>

              <Button
                component={Link}
                href="/auth/login"
                variant="filled"
                size="sm"
                mt="md"
              >
                Перейти ко входу
              </Button>
            </Stack>
          ) : (
            <>
              <Title order={2} ta="center" mb={12} fz={22}>
                Установка нового пароля
              </Title>

              <Text c="dimmed" ta="center" mb={24} fz={14}>
                Придумайте новый надёжный пароль для вашей учётной записи.
              </Text>

              {!token && (
                <Alert
                  icon={<IconAlertCircle size={18} />}
                  color="red"
                  mb={20}
                  title="Ссылка недействительна"
                  radius="md"
                >
                  Токен сброса пароля отсутствует в адресе страницы. Пожалуйста, запросите сброс пароля заново.
                </Alert>
              )}

              {error && (
                <Alert
                  icon={<IconAlertCircle size={18} />}
                  color="red"
                  mb={20}
                  title="Ошибка"
                  radius="md"
                >
                  {error}
                </Alert>
              )}

              <form onSubmit={handleSubmit}>
                <Stack gap={16}>
                  <PasswordInput
                    label="Новый пароль"
                    placeholder="Введите новый пароль"
                    required
                    size="md"
                    radius="md"
                    value={password}
                    onChange={(e) => setPassword(e.currentTarget.value)}
                  />

                  <PasswordInput
                    label="Подтверждение пароля"
                    placeholder="Повторите новый пароль"
                    required
                    size="md"
                    radius="md"
                    value={confirmPassword}
                    onChange={(e) => setConfirmPassword(e.currentTarget.value)}
                  />

                  <Button
                    type="submit"
                    fullWidth
                    size="md"
                    radius="md"
                    loading={loading}
                    disabled={!token}
                    mt={8}
                  >
                    Сохранить новый пароль
                  </Button>
                </Stack>
              </form>

              <Text c="dimmed" ta="center" mt={24} fz={14}>
                Вспомнили старый пароль?{" "}
                <Anchor
                  component={Link}
                  href="/auth/login"
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

export default ResetPasswordPage;
