import { Container, Title, Text, Button, Stack, Paper, Code, Group } from "@mantine/core";
import Link from "next/link";
import type { ApiError } from "@/lib/api";
import { RefreshButton } from "./RefreshButton";

type Props = {
  error: ApiError;
};

function getErrorDisplay(status: number): { title: string; description: string } {
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
        description: "При обработке запроса произошла ошибка",
      };
  }
}

export function ErrorDisplay({ error }: Props) {
  const display = getErrorDisplay(error.status);

  return (
    <Container size="sm" py="xl">
      <Paper p="xl" radius="md" withBorder>
        <Stack align="center" gap="lg">
          <Title order={1}>{display.title}</Title>
          
          <Text c="dimmed" ta="center">
            {display.description}
          </Text>

          {error.requestId && (
            <Code style={{ fontSize: "0.75rem" }}>{error.requestId}</Code>
          )}

          <Group>
            {error.status === 401 ? (
              <Button component={Link} href="/auth/login" variant="filled">
                Войти
              </Button>
            ) : (
              <RefreshButton />
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

