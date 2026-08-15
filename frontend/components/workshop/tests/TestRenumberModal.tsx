"use client";

import {Alert, Button, Group, Modal, Stack, Text} from "@mantine/core";

import type {ReactNode} from "react";

type Props = {
  opened: boolean;
  onClose: () => void;
  onConfirm: () => void;
  isLoading?: boolean;
  gapNumbers: number[];
};

export const TestRenumberModal = ({
  opened,
  onClose,
  onConfirm,
  isLoading,
  gapNumbers,
}: Props): ReactNode => {
  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title={<Text fw={600}>Перенумерация тестов (1..N)</Text>}
      size="md"
    >
      <Stack gap="md">
        <Text size="sm">
          В списке тестов обнаружены пропуски в нумерации{" "}
          {gapNumbers.length > 0 && `(отсутствуют: ${gapNumbers.join(", ")})`}.
        </Text>

        <Alert color="blue" title="Как работает перенумерация">
          Все оставшиеся тесты будут последовательно переименованы в файлы{" "}
          <code>01.in / 01.out</code>, <code>02.in / 02.out</code> и т.д.
          Диапазоны в сабтасках будут автоматически пересчитаны, <b>принадлежность тестов к их сабтаскам сохранится.</b>
        </Alert>

        <Group justify="flex-end" gap="sm">
          <Button variant="default" onClick={onClose} disabled={isLoading}>
            Оставить как есть
          </Button>
          <Button color="blue" onClick={onConfirm} loading={isLoading}>
            Перенумеровать (1..N)
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
};
