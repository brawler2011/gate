"use client";

import { Button, Group, Modal, Stack, Text } from "@mantine/core";
import { useState } from "react";

interface DeleteBlogPostModalProps {
  opened: boolean;
  onClose: () => void;
  post: {
    id: string;
    title: string;
  };
  onSubmit: () => Promise<void>;
}

export function DeleteBlogPostModal({
  opened,
  onClose,
  post,
  onSubmit,
}: DeleteBlogPostModalProps) {
  const [loading, setLoading] = useState(false);

  const handleSubmit = async () => {
    try {
      setLoading(true);
      await onSubmit();
      onClose();
    } catch (error) {
      console.error("Failed to delete post:", error);
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal
      opened={opened}
      onClose={onClose}
      title="Удалить пост"
      centered
      size="md"
      radius="md"
      overlayProps={{ backgroundOpacity: 0.4 }}
    >
      <Stack gap="md">
        <div>
          <Text size="sm" c="dimmed" mb={4}>
            Пост
          </Text>
          <Text size="md" fw={500}>
            {post.title}
          </Text>
        </div>

        <Text size="sm" c="dimmed">
          Это действие нельзя отменить. Пост будет удалён безвозвратно.
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


