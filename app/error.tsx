"use client";

import { Container, Title, Text, Button, Stack, Paper, Code, Group } from "@mantine/core";
import Link from "next/link";

type ErrorProps = {
  error: Error & { digest?: string };
  reset: () => void;
};

function parseError(message: string): { status: number | null; text: string; requestId: string | null } {
  // Parse "401: message [request-id]" format
  const match = message.match(/^(\d{3}):\s*(.+?)(?:\s*\[([^\]]+)\])?$/);
  if (match) {
    return {
      status: parseInt(match[1], 10),
      text: match[2].trim(),
      requestId: match[3] || null,
    };
  }
  return { status: null, text: message, requestId: null };
}

function getErrorInfo(status: number | null, text: string): { title: string; description: string } {
  switch (status) {
    case 401:
      return {
        title: "Требуется авторизация",
        description: "Войдите в систему для доступа к этой странице",
      };
    case 403:
      return {
        title: "Доступ запрещён",
        description: "У вас нет прав для просмотра этой страницы",
      };
    case 404:
      return {
        title: "Не найдено",
        description: "Запрашиваемая страница не существует",
      };
    case 500:
      return {
        title: "Ошибка сервера",
        description: "Внутренняя ошибка сервера. Попробуйте позже",
      };
    default:
      return {
        title: "Произошла ошибка",
        description: text || "При обработке запроса произошла ошибка",
      };
  }
}

export default function Error({ error, reset }: ErrorProps) {
  const parsed = parseError(error.message);
  const info = getErrorInfo(parsed.status, parsed.text);

  return (
    <Container size="sm" py="xl">
      <Paper p="xl" radius="md" withBorder>
        <Stack align="center" gap="lg">
          <Title order={1}>{info.title}</Title>
          
          <Text c="dimmed" ta="center">
            {info.description}
          </Text>

          {parsed.requestId && (
            <Code style={{ fontSize: "0.75rem" }}>{parsed.requestId}</Code>
          )}

          <Group>
            {parsed.status === 401 ? (
              <Button component={Link} href="/auth/login" variant="filled">
                Войти
              </Button>
            ) : (
              <Button onClick={() => window.location.reload()} variant="filled">
                Обновить страницу
              </Button>
            )}
            <Button component={Link} href="/" variant="light">
              На главную
            </Button>
          </Group>
        </Stack>
      </Paper>
    </Container>
  );
}

