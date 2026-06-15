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
import { IconAlertCircle } from "@tabler/icons-react";
import Image from "next/image";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useState } from "react";
import { registerAction } from "@lib/auth-actions";

export default function RegistrationPage() {
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
}

function RegistrationPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const returnTo = searchParams.get("return_to") || "/";

  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const result = await registerAction(username, email, password);
      if (result.success) {
        router.push(returnTo);
        router.refresh();
      } else {
        setError(result.error || "Ошибка при регистрации");
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
        style={{ width: "min(550px, calc(100vw - 2rem))" }}
      >
        <Link href="/" style={{ textDecoration: "none", color: "inherit" }}>
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
          style={{ width: "100%" }}
        >
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
        </Paper>
      </Stack>
    </Box>
  );
}
