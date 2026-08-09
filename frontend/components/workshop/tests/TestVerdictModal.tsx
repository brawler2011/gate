"use client";

import {
  Badge,
  Box,
  Button,
  Code,
  Group,
  Modal,
  Stack,
  Text,
} from "@mantine/core";

type Props = {
  opened: boolean;
  onClose: () => void;
  title: string;
  verdictBadge?: { label: string; color: string };
  time?: number;
  memory?: number;
  message?: string;
  error?: string;
};

export const TestVerdictModal = ({
  opened,
  onClose,
  title,
  verdictBadge,
  time,
  memory,
  message,
  error,
}: Props) => {
  return (
    <Modal opened={opened} onClose={onClose} title={<Text fw={600}>{title}</Text>} size="md">
      <Stack gap="md">
        {verdictBadge && (
          <Group gap="xs">
            <Text fw={500} size="sm">Вердикт:</Text>
            <Badge color={verdictBadge.color} variant="filled">
              {verdictBadge.label}
            </Badge>
          </Group>
        )}

        {(time !== undefined || memory !== undefined) && (
          <Group gap="lg">
            {time !== undefined && (
              <Text size="sm">
                <b>Время:</b> {time} мс
              </Text>
            )}
            {memory !== undefined && (
              <Text size="sm">
                <b>Память:</b> {(memory / (1024 * 1024)).toFixed(2)} МБ
              </Text>
            )}
          </Group>
        )}

        {message && (
          <Box>
            <Text fw={500} size="sm" mb={4}>
              Сообщение / Вывод:
            </Text>
            <Code block style={{whiteSpace: "pre-wrap"}}>
              {message}
            </Code>
          </Box>
        )}

        {error && (
          <Box>
            <Text fw={500} size="sm" c="red" mb={4}>
              Детали ошибки:
            </Text>
            <Code block color="red" style={{whiteSpace: "pre-wrap"}}>
              {error}
            </Code>
          </Box>
        )}

        <Group justify="flex-end">
          <Button variant="default" onClick={onClose}>
            Закрыть
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
};
