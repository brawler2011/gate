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
  return_to?: string;
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

export default function RegistrationPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const flowId = searchParams.get("flow");
  const returnTo = searchParams.get("return_to");

  const [flow, setFlow] = useState<FlowData | null>(null);
  const [username, setUsername] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (!flowId) {
      window.location.href = `/api/.ory/self-service/registration/browser${returnTo ? `?return_to=${encodeURIComponent(returnTo)}` : ""}`;
      return;
    }

    fetch(`/api/.ory/self-service/registration/flows?id=${flowId}`, {
      credentials: "include",
    })
      .then((res) => {
        if (res.status === 410 || res.status === 404 || res.status === 403) {
          window.location.href = `/api/.ory/self-service/registration/browser${returnTo ? `?return_to=${encodeURIComponent(returnTo)}` : ""}`;
          return null;
        }
        return res.json();
      })
      .then((data) => {
        if (data) setFlow(data);
      })
      .catch(() => {
        window.location.href = `/api/.ory/self-service/registration/browser${returnTo ? `?return_to=${encodeURIComponent(returnTo)}` : ""}`;
      });
  }, [flowId, returnTo]);

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
    
    // Get return_to from flow object, fallback to URL param, then to "/"
    const targetReturnTo = flow.return_to || returnTo || "/";
    
    try {
      const response = await fetch(`/api/.ory/self-service/registration?flow=${flowId}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Accept: "application/json",
        },
        body: JSON.stringify({
          method: "password",
          traits: {
            username,
            email,
          },
          password,
          csrf_token: csrfToken,
        }),
        credentials: "include",
      });

      const data = await response.json();

      // Check if registration was successful (session exists)
      if (response.ok && data.session) {
        // Kratos successfully registered, redirect to return_to URL from flow
        router.push(targetReturnTo);
        router.refresh();
        return;
      }
      
      // If response is ok but no session, still redirect (edge case)
      if (response.ok) {
        router.push(targetReturnTo);
        router.refresh();
        return;
      }

      if (response.status === 410 || response.status === 404 || response.status === 403) {
        window.location.href = `/api/.ory/self-service/registration/browser${returnTo ? `?return_to=${encodeURIComponent(returnTo)}` : ""}`;
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
          setError("Не удалось создать аккаунт");
        }
      } else {
        setError("Не удалось создать аккаунт");
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
      <Stack align="center" gap={32} style={{ width: "min(550px, calc(100vw - 2rem))" }}>
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

        <Paper radius="md" p={32} withBorder shadow="sm" style={{ width: "100%" }}>
          <Title order={2} ta="center" mb={24} fz={22}>
            Регистрация
          </Title>

          {error && (
            <Alert
              icon={<IconAlertCircle size={18} />}
              color="red"
              mb={20}
              title="Ошибка регистрации"
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
                placeholder="Минимум 8 символов"
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
                Создать аккаунт
              </Button>
            </Stack>
          </form>

          <Text c="dimmed" ta="center" mt={24} fz={14}>
            Уже есть аккаунт?{" "}
            <Anchor component={Link} href={`/auth/login${returnTo ? `?return_to=${encodeURIComponent(returnTo)}` : ""}`} fz={14} fw={600}>
              Войти
            </Anchor>
          </Text>
        </Paper>
      </Stack>
    </Box>
  );
}
