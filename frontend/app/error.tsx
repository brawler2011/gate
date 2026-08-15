"use client";

import {Button, Code, Container, Paper, Stack, Text, Title} from "@mantine/core";
import Link from "next/link";
import {useEffect} from "react";

import type {ReactNode} from "react";

type ErrorProps = {
  error: Error & { digest?: string };
  reset: () => void;
};

const Error = ({error, reset}: ErrorProps): ReactNode => {
  useEffect(() => {
    console.error("Unhandled page error:", error);
  }, [error]);

  return (
    <Container size="sm" py="xl">
      <Paper p="xl" radius="md" withBorder>
        <Stack align="center" gap="lg">
          <Title order={1}>Произошла ошибка</Title>

          <Text c="dimmed" ta="center">
            Не удалось загрузить данные с сервера. Попробуйте обновить страницу или повторить попытку позже.
          </Text>

          {error.digest ? (
            <Stack gap="xs" align="center">
              <Text size="sm" c="dimmed">
                ID ошибки:
              </Text>
              <Code>{error.digest}</Code>
            </Stack>
          ) : null}

          <Stack gap="sm" align="center">
            <Button variant="light" onClick={() => reset()}>
              Попробовать снова
            </Button>
            <Link href="/" style={{textDecoration: "none"}}>
              <Button variant="subtle" color="gray">
                На главную
              </Button>
            </Link>
          </Stack>
        </Stack>
      </Paper>
    </Container>
  );
};

export default Error;
