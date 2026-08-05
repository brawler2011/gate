import { Container, Title, Text, Button, Stack, Paper } from "@mantine/core";
import Link from "next/link";

import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Доступ запрещен",
};

const ForbiddenPage = () => {
  return (
    <Container size="sm" py="xl">
      <Paper p="xl" radius="md" withBorder>
        <Stack align="center" gap="lg">
          <Title order={1} c="red">403</Title>
          <Title order={2}>Доступ запрещен</Title>
          
          <Text c="dimmed" ta="center">
            У вас нет прав для просмотра этого ресурса.
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
};

export default ForbiddenPage;
