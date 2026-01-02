import { Container, Title, Text, Button, Stack, Paper } from "@mantine/core";
import Link from "next/link";
import type { Metadata } from "next";

// Next.js 15 автоматически кэширует not-found.tsx при билде
// Эта страница статическая и будет отдаваться из кеша
export const metadata: Metadata = {
  title: '404 - Страница не найдена',
  description: 'Запрашиваемая страница не существует',
  robots: 'noindex, nofollow',
};

export default function NotFound() {
  return (
    <Container size="sm" py="xl">
      <Paper p="xl" radius="md" withBorder>
        <Stack align="center" gap="lg">
          <Title order={1} c="dimmed">404</Title>
          <Title order={2}>Страница не найдена</Title>
          
          <Text c="dimmed" ta="center">
            Запрашиваемая страница не существует или была удалена.
          </Text>

          <Link href="/" style={{ textDecoration: 'none' }}>
            <Button variant="filled">
              Вернуться на главную
            </Button>
          </Link>
        </Stack>
      </Paper>
    </Container>
  );
}

