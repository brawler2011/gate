"use client";

import { useState, useEffect } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  TextInput,
  PasswordInput,
  Button,
  Paper,
  Title,
  Text,
  Anchor,
  Stack,
  Alert,
  Center,
  Loader,
  Group,
  Box,
} from "@mantine/core";
import { IconAlertCircle } from "@tabler/icons-react";
import Link from "next/link";
import Image from "next/image";

type FlowData = {
  id: string;
  ui: {
    action: string;
    method: string;
    nodes: Array<{
      attributes: {
        name?: string;
        value?: string;
        type?: string;
      };
      messages?: Array<{ text: string }>;
    }>;
    messages?: Array<{ text: string }>;
  };
};

export default function LoginPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const flowId = searchParams.get("flow");

  const [flow, setFlow] = useState<FlowData | null>(null);
  const [identifier, setIdentifier] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!flowId) {
      window.location.href = "/api/.ory/self-service/login/browser";
      return;
    }

    fetch(`/api/.ory/self-service/login/flows?id=${flowId}`, {
      credentials: "include",
    })
      .then((res) => {
        if (res.status === 410 || res.status === 404 || res.status === 403) {
          window.location.href = "/api/.ory/self-service/login/browser";
          return null;
        }
        return res.json();
      })
      .then((data) => {
        if (data) setFlow(data);
      })
      .catch(() => {
        window.location.href = "/api/.ory/self-service/login/browser";
      });
  }, [flowId]);

  if (!flowId || !flow) {
    return (
      <Center h="100vh">
        <Loader size="lg" />
      </Center>
    );
  }

  const csrfNode = flow.ui.nodes.find(
    (node) => node.attributes.name === "csrf_token"
  );
  const csrfToken = csrfNode?.attributes.value || "";

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    setLoading(true);

    try {
      const response = await fetch(`/api/.ory/self-service/login?flow=${flowId}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: JSON.stringify({
          method: "password",
          identifier,
          password,
          csrf_token: csrfToken,
        }),
        credentials: "include",
      });

      if (response.ok) {
        router.push("/");
        router.refresh();
        return;
      }

      const data = await response.json();

      if (response.status === 410 || response.status === 404 || response.status === 403) {
        window.location.href = "/api/.ory/self-service/login/browser";
        return;
      }

      if (data.ui?.messages) {
        const messages = data.ui.messages.map((m: { text: string }) => m.text).join(". ");
        setError(messages);
      } else if (data.ui?.nodes) {
        const fieldErrors: string[] = [];
        for (const node of data.ui.nodes) {
          if (node.messages?.length > 0) {
            fieldErrors.push(...node.messages.map((m: { text: string }) => m.text));
          }
        }
        if (fieldErrors.length > 0) {
          setError(fieldErrors.join(". "));
        } else {
          setError("Неверный логин или пароль");
        }
      } else {
        setError("Неверный логин или пароль");
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
        padding: "2rem",
      }}
    >
      <Stack align="center" gap={32}>
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

        <Paper radius="md" p={32} withBorder w={550} shadow="sm">
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
              {error}
            </Alert>
          )}

          <form onSubmit={handleSubmit}>
            <Stack gap={16}>
              <TextInput
                label="Email или имя пользователя"
                placeholder="Введите email"
                required
                size="md"
                radius="md"
                value={identifier}
                onChange={(e) => setIdentifier(e.currentTarget.value)}
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
                Войти
              </Button>
            </Stack>
          </form>

          <Text c="dimmed" ta="center" mt={24} fz={14}>
            Ещё нет аккаунта?{" "}
            <Anchor component={Link} href="/auth/registration" fz={14} fw={600}>
              Зарегистрироваться
            </Anchor>
          </Text>
        </Paper>
      </Stack>
    </Box>
  );
}
