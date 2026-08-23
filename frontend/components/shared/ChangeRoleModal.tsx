"use client";

import {Button, Modal, Select, Stack, Text} from "@mantine/core";
import {useEffect, useState, type ReactNode} from "react";

export interface RoleOption {
  label: string;
  value: string;
}

const DEFAULT_ROLE_OPTIONS: RoleOption[] = [
  {label: "Участник", value: "participant"},
  {label: "Модератор", value: "moderator"},
];

export interface ChangeRoleModalProps {
  opened: boolean;
  onClose: () => void;
  participant: {
    username: string;
    userId?: string;
  };
  currentRole: string;
  onSubmit: (newRole: string) => Promise<void>;
  roleOptions?: RoleOption[];
  title?: string;
  entityLabel?: string;
  selectLabel?: string;
}

export const ChangeRoleModal = ({
  opened,
  onClose,
  participant,
  currentRole,
  onSubmit,
  roleOptions = DEFAULT_ROLE_OPTIONS,
  title = "Изменить роль",
  entityLabel = "Участник",
  selectLabel = "Новая роль",
}: ChangeRoleModalProps): ReactNode => {
  const [selectedRole, setSelectedRole] = useState<string>(currentRole);
  const [loading, setLoading] = useState(false);

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
      title={title}
      centered
      size="md"
      radius="md"
      overlayProps={{backgroundOpacity: 0.4}}
    >
      <Stack gap="md">
        <div>
          <Text size="sm" c="dimmed" mb={4}>
            {entityLabel}
          </Text>
          <Text size="md" fw={500}>
            {participant.username}
          </Text>
        </div>

        <Select
          label={selectLabel}
          placeholder="Выберите роль"
          value={selectedRole}
          onChange={(value) => setSelectedRole(value || currentRole)}
          data={roleOptions}
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
};
