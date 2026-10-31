import {Container, Title, Text, Button, Stack, Paper} from "@mantine/core";
import {Link} from "react-router-dom";

import type {ReactNode} from "react";

const NotFound = (): ReactNode => {
  return (
    <Container size="sm" py="xl">
      <Paper p="xl" radius="md" withBorder>
        <Stack align="center" gap="lg">
          <Title order={1} c="dimmed">404</Title>
          <Title order={2}>Страница не найдена</Title>

          <Text c="dimmed" ta="center">
            Запрашиваемая страница не существует или была удалена.
          </Text>

          <Link to="/" style={{textDecoration: 'none'}}>
            <Button variant="filled">
              Вернуться на главную
            </Button>
          </Link>
        </Stack>
      </Paper>
    </Container>
  );
};

export default NotFound;
