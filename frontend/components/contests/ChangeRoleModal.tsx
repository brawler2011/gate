"use client";

import { Button, Modal, Select, Stack, Text } from "@mantine/core";
import { useEffect, useState } from "react";

const ROLE_OPTIONS = [
  { label: "Участник", value: "participant" },
  { label: "Модератор", value: "moderator" },
];

interface ChangeRoleModalProps {
  opened: boolean;
  onClose: () => void;
  participant: {
    username: string;
    userId: string;
  };
  currentRole: string;
  onSubmit: (newRole: string) => Promise<void>;
}

export function ChangeRoleModal({
  opened,
  onClose,
  participant,
  currentRole,
  onSubmit,
}: ChangeRoleModalProps) {
  const [selectedRole, setSelectedRole] = useState<string>(currentRole);
  const [loading, setLoading] = useState(false);

  // Синхронизируем selectedRole с currentRole при открытии модального окна
  useEffect(() => {
    if (opened) {
      setSelectedRole(currentRole);
    }
  }, [opened, currentRole]);

  const handleSubmit = async () => {
    try {
      setLoading(true);
      await onSubmit(selectedRole);
      onClose();
    } catch (error) {
      console.error("Failed to change role:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Изменить роль"
      centered
      size="md"
      radius="md"
      overlayProps={{ backgroundOpacity: 0.4 }}
    >
      <Stack gap="md">
        <div>
          <Text size="sm" c="dimmed" mb={4}>
            Участник
          </Text>
          <Text size="md" fw={500}>
            {participant.username}
          </Text>
        </div>

        <Select
          label="Новая роль"
          placeholder="Выберите роль"
          value={selectedRole}
          onChange={(value) => setSelectedRole(value || currentRole)}
          data={ROLE_OPTIONS}
          required
        />

        <Button
          onClick={handleSubmit}
          loading={loading}
          disabled={selectedRole === currentRole || !selectedRole}
          fullWidth
        >
          Применить
        </Button>
      </Stack>
    </Modal>
  );
}

