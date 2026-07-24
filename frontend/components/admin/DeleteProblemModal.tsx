"use client";

import { Button, Group, Modal, Stack, Text } from "@mantine/core";
import { useState } from "react";

interface DeleteProblemModalProps {
  opened: boolean;
  onClose: () => void;
  problem: {
    id: string;
    title: string;
  };
  onSubmit: () => Promise<void>;
}

export function DeleteProblemModal({
  opened,
  onClose,
  problem,
  onSubmit,
}: DeleteProblemModalProps) {
  const [loading, setLoading] = useState(false);

  const handleSubmit = async () => {
    try {
      setLoading(true);
      await onSubmit();
      onClose();
    } catch (error) {
      console.error("Failed to delete problem:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Удалить задачу"
      centered
      size="md"
      radius="md"
      overlayProps={{ backgroundOpacity: 0.4 }}
    >
      <Stack gap="md">
        <div>
          <Text size="sm" c="dimmed" mb={4}>
            Задача
          </Text>
          <Text size="md" fw={500}>
            {problem.title}
          </Text>
        </div>

        <Text size="sm" c="dimmed">
          Это действие нельзя отменить. Все данные задачи, включая тесты, решения и генераторы, будут безвозвратно удалены.
        </Text>

        <Group justify="flex-end" gap="sm">
          <Button variant="default" onClick={onClose} disabled={loading}>
            Отмена
          </Button>
          <Button color="red" onClick={handleSubmit} loading={loading}>
            Удалить
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
}
