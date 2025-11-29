"use client";

import { Container, Title, Text, Button, Stack, Paper } from "@mantine/core";
import { useEffect } from "react";

type ErrorProps = {
  error: Error & { digest?: string };
  reset: () => void;
};

export default function Error({ error, reset }: ErrorProps) {
  useEffect(() => {
    // Log the error to an error reporting service
    console.error(error);
  }, [error]);

  return (
    <Container size="sm" py="xl">
      <Paper p="xl" radius="md" withBorder>
        <Stack align="center" gap="lg">
          <Title order={1}>Произошла ошибка</Title>
          
          <Text c="dimmed" ta="center">
            При обработке вашего запроса произошла ошибка. 
            Пожалуйста, попробуйте снова.
          </Text>

          {error.digest && (
            <Text size="xs" c="dimmed">
              ID: {error.digest}
            </Text>
          )}

          <Button onClick={reset} variant="filled">
            Попробовать снова
          </Button>
        </Stack>
      </Paper>
    </Container>
  );
}

