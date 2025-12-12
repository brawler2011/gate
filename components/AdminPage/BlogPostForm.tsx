"use client";

import {
  Button,
  FileInput,
  Group,
  Modal,
  Stack,
  Textarea,
  TextInput,
} from "@mantine/core";
import { IconPhoto, IconUpload } from "@tabler/icons-react";
import { useState } from "react";
import type { PostModel } from "../../../contracts/gateway/v1";

interface BlogPostFormProps {
  opened: boolean;
  onClose: () => void;
  post?: PostModel | null;
  onSubmit: (data: {
    title: string;
    description: string;
    text: string;
    preview_image?: File | null;
  }) => Promise<void>;
}

export function BlogPostForm({
  opened,
  onClose,
  post,
  onSubmit,
}: BlogPostFormProps) {
  const [loading, setLoading] = useState(false);
  const [title, setTitle] = useState(post?.title || "");
  const [description, setDescription] = useState(post?.description || "");
  const [text, setText] = useState(post?.text || "");
  const [previewImage, setPreviewImage] = useState<File | null>(null);
  const [errors, setErrors] = useState<{ title?: string }>({});

  const isEditing = !!post;

  const handleSubmit = async () => {
    // Validate
    const newErrors: { title?: string } = {};
    if (!title.trim()) {
      newErrors.title = "Заголовок обязателен";
    }
    if (Object.keys(newErrors).length > 0) {
      setErrors(newErrors);
      return;
    }

    try {
      setLoading(true);
      await onSubmit({
        title: title.trim(),
        description: description.trim(),
        text: text.trim(),
        preview_image: previewImage,
      });
      handleClose();
    } catch (error) {
      console.error("Failed to save post:", error);
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setTitle("");
    setDescription("");
    setText("");
    setPreviewImage(null);
    setErrors({});
    onClose();
  };

  // Reset form when modal opens with new data
  const handleModalOpen = () => {
    setTitle(post?.title || "");
    setDescription(post?.description || "");
    setText(post?.text || "");
    setPreviewImage(null);
    setErrors({});
  };

  return (
    <Modal
      opened={opened}
      onClose={handleClose}
      title={isEditing ? "Редактировать пост" : "Создать пост"}
      centered
      size="lg"
      radius="md"
      overlayProps={{ backgroundOpacity: 0.4 }}
      onOpen={handleModalOpen}
    >
      <Stack gap="md">
        <TextInput
          label="Заголовок"
          placeholder="Введите заголовок поста"
          value={title}
          onChange={(e) => {
            setTitle(e.currentTarget.value);
            if (errors.title) setErrors({});
          }}
          error={errors.title}
          required
        />

        <Textarea
          label="Описание"
          placeholder="Краткое описание поста"
          value={description}
          onChange={(e) => setDescription(e.currentTarget.value)}
          minRows={2}
          maxRows={4}
          autosize
        />

        <Textarea
          label="Текст (Markdown)"
          placeholder="Основной текст поста в формате Markdown"
          value={text}
          onChange={(e) => setText(e.currentTarget.value)}
          minRows={10}
          maxRows={20}
          autosize
          styles={{
            input: {
              fontFamily: "monospace",
            },
          }}
        />

        <FileInput
          label="Изображение для превью"
          placeholder="Выберите изображение"
          leftSection={<IconPhoto size={16} />}
          value={previewImage}
          onChange={setPreviewImage}
          accept="image/png,image/jpeg,image/jpg,image/gif,image/webp"
          clearable
        />

        <Group justify="flex-end" gap="sm" mt="md">
          <Button variant="default" onClick={handleClose} disabled={loading}>
            Отмена
          </Button>
          <Button
            onClick={handleSubmit}
            loading={loading}
            leftSection={<IconUpload size={16} />}
          >
            {isEditing ? "Сохранить" : "Создать"}
          </Button>
        </Group>
      </Stack>
    </Modal>
  );
}
