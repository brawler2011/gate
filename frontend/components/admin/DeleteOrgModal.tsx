"use client";

import { Button, Group, Modal, Stack, Text } from "@mantine/core";
import { useState } from "react";

interface DeleteOrgModalProps {
  opened: boolean;
  onClose: () => void;
  org: {
    id: string;
    name: string;
  };
  onSubmit: () => Promise<void>;
}

export function DeleteOrgModal({
  opened,
  onClose,
  org,
  onSubmit,
}: DeleteOrgModalProps) {
  const [loading, setLoading] = useState(false);

  const handleSubmit = async () => {
    try {
      setLoading(true);
      await onSubmit();
      onClose();
    } catch (error) {
      console.error("Failed to delete organization:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Удалить организацию"
      centered
      size="md"
      radius="md"
      overlayProps={{ backgroundOpacity: 0.4 }}
    >
      <Stack gap="md">
        <div>
          <Text size="sm" c="dimmed" mb={4}>
            Организация
          </Text>
          <Text size="md" fw={500}>
            {org.name}
          </Text>
        </div>

        <Text size="sm" c="dimmed">
          Это действие нельзя отменить. Все данные организации, включая команды, контесты и участников, будут безвозвратно удалены.
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
