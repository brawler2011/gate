import { Container, Title, Text, Button, Stack, Paper, Code } from "@mantine/core";
import Link from "next/link";

type PageProps = {
  searchParams: Promise<{ id?: string }>;
};

export default async function ErrorPage({ searchParams }: PageProps) {
  const params = await searchParams;
  const errorId = params.id;

  return (
    <Container size="sm" py="xl">
      <Paper p="xl" radius="md" withBorder>
        <Stack align="center" gap="lg">
          <Title order={1}>Произошла ошибка</Title>
          
          <Text c="dimmed" ta="center">
            При обработке вашего запроса произошла ошибка. 
            Пожалуйста, попробуйте снова или вернитесь на главную страницу.
          </Text>

          {errorId && (
            <Stack gap="xs" align="center">
              <Text size="sm" c="dimmed">ID ошибки:</Text>
              <Code>{errorId}</Code>
            </Stack>
          )}

          <Button component={Link} href="/" variant="light">
            Вернуться на главную
          </Button>
        </Stack>
      </Paper>
    </Container>
  );
}

