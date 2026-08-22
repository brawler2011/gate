"use client";

import {
  Alert,
  Anchor,
  Box,
  Button,
  Group,
  Paper,
  Stack,
  Text,
  TextInput,
  ThemeIcon,
  Title,
} from "@mantine/core";
import {IconAlertCircle, IconMailForward} from "@tabler/icons-react";
import Image from "next/image";
import Link from "next/link";
import {useState, type ReactNode} from "react";

import {api} from "@/lib/api";

const ForgotPasswordPage = (): ReactNode => {
  const [identifier, setIdentifier] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [submitted, setSubmitted] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const [err] = await api.forgotPassword({
        requestBody: {identifier},
      });
      if (!err) {
        setSubmitted(true);
      } else {
        setError(err?.message || "Ошибка при отправке запроса");
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
          {submitted ? (
            <Stack align="center" gap="md" ta="center">
              <ThemeIcon size={64} radius="xl" color="blue" variant="light">
                <IconMailForward size={36} />
              </ThemeIcon>

              <Title order={2} fz={22}>
                Проверьте почту
              </Title>

              <Text c="dimmed" fz={15}>
                Если аккаунт с указанным логином или email существует, мы отправили на него ссылку для восстановления пароля.
              </Text>

              <Text c="dimmed" fz={13}>
                Ссылка действительна в течение 1 часа.
              </Text>

              <Button
                component={Link}
                href="/auth/login"
                variant="filled"
                size="sm"
                mt="md"
              >
                Вернуться ко входу
              </Button>
            </Stack>
          ) : (
            <>
              <Title order={2} ta="center" mb={12} fz={22}>
                Восстановление пароля
              </Title>

              <Text c="dimmed" ta="center" mb={24} fz={14}>
                Введите имя пользователя или email, указанный при регистрации, чтобы получить ссылку для сброса пароля.
              </Text>

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
                  <TextInput
                    label="Email или имя пользователя"
                    placeholder="Введите email или имя пользователя"
                    required
                    size="md"
                    radius="md"
                    value={identifier}
                    onChange={(e) => setIdentifier(e.currentTarget.value)}
                  />

                  <Button
                    type="submit"
                    fullWidth
                    size="md"
                    radius="md"
                    loading={loading}
                    mt={8}
                  >
                    Отправить ссылку для сброса
                  </Button>
                </Stack>
              </form>

              <Text c="dimmed" ta="center" mt={24} fz={14}>
                Вспомнили пароль?{" "}
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

export default ForgotPasswordPage;
