import { Container, Title, Text, Button, Stack, Paper } from "@mantine/core";
import Link from "next/link";

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

          <Button component={Link} href="/" variant="filled">
            Вернуться на главную
          </Button>
        </Stack>
      </Paper>
    </Container>
  );
}

