"use client";

import { Button, Group, Modal, Stack, Text } from "@mantine/core";
import { useState } from "react";

interface DeleteContestModalProps {
  opened: boolean;
  onClose: () => void;
  contest: {
    id: string;
    title: string;
  };
  onSubmit: () => Promise<void>;
}

export function DeleteContestModal({
  opened,
  onClose,
  contest,
  onSubmit,
}: DeleteContestModalProps) {
  const [loading, setLoading] = useState(false);

  const handleSubmit = async () => {
    try {
      setLoading(true);
      await onSubmit();
      onClose();
    } catch (error) {
      console.error("Failed to delete contest:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Удалить контест"
      centered
      size="md"
      radius="md"
      overlayProps={{ backgroundOpacity: 0.4 }}
    >
      <Stack gap="md">
        <div>
          <Text size="sm" c="dimmed" mb={4}>
            Контест
          </Text>
          <Text size="md" fw={500}>
            {contest.title}
          </Text>
        </div>

        <Text size="sm" c="dimmed">
          Это действие нельзя отменить. Все данные контеста будут удалены.
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

