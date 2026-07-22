"use client";

import { createPost, deletePost, patchPost } from "@/lib/actions";
import {
  Button,
  Center,
  Container,
  Group,
  Skeleton,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { IconPlus } from "@tabler/icons-react";
import { useState } from "react";
import type { PostModel } from "@contracts/core/v1";
import { NextPagination } from '@/components/shared/Pagination';
import { StatusMessage } from '@/components/shared/StatusMessage';
import { AdminBlogsSearchInput } from "./AdminBlogsSearchInput";
import { AdminBlogsTable } from "./AdminBlogsTable";
import { BlogPostForm } from "./BlogPostForm";
import useSWR from "swr";

type AdminBlogsContentProps = {
  page: number;
  search?: string;
};

export function AdminBlogsContent({ page, search }: AdminBlogsContentProps) {
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  // Form modal state
  const [formOpened, setFormOpened] = useState(false);
  const [editingPost, setEditingPost] = useState<PostModel | null>(null);

  const { data, error, isLoading, mutate } = useSWR(
    `/api/admin/blogs?page=${page}`,
    async (url) => {
      const res = await fetch(url);
      if (!res.ok) {
        const errData = await res.json().catch(() => ({}));
        throw new Error(errData.message || "Не удалось загрузить посты");
      }
      return res.json();
    }
  );

  const posts = data?.posts || [];
  const pagination = data?.pagination || { total: 0, page: page };

  const handleCreatePost = () => {
    setEditingPost(null);
    setFormOpened(true);
  };

  const handleEditPost = (post: PostModel) => {
    setEditingPost(post);
    setFormOpened(true);
  };

  const handleFormSubmit = async (data: {
    title: string;
    description: string;
    text: string;
    preview_image?: File | null;
  }) => {
    const formData: {
      title?: string;
      description?: string;
      text?: string;
      preview_image?: Blob;
    } = {
      title: data.title,
      description: data.description,
      text: data.text,
    };

    if (data.preview_image) {
      formData.preview_image = data.preview_image;
    }

    if (editingPost) {
      // Update existing post
      const [err] = await patchPost(editingPost.id || "", formData);
      if (err) {
        notifications.show({
          title: "Ошибка",
          message: err.message || "Не удалось обновить пост",
          color: "red",
        });
        throw new Error(err.message);
      }
      setStatusMessage({
        type: "success",
        message: "Пост успешно обновлён",
      });
    } else {
      // Create new post
      const [err] = await createPost(formData);
      if (err) {
        notifications.show({
          title: "Ошибка",
          message: err.message || "Не удалось создать пост",
          color: "red",
        });
        throw new Error(err.message);
      }
      setStatusMessage({
        type: "success",
        message: "Пост успешно создан",
      });
    }

    mutate();
  };

  const handleDeletePost = async (postId: string) => {
    const [err] = await deletePost(postId);
    
    if (err) {
      console.error("Error deleting post:", err);
      notifications.show({
        title: "Ошибка",
        message: err.message || "Не удалось удалить пост",
        color: "red",
      });
      throw new Error(err.message);
    }

    setStatusMessage({
      type: "success",
      message: "Пост успешно удалён",
    });
    // Reload posts after deletion
    mutate();
  };

  if (error) {
    return (
      <Container size="xl" py="md">
        <Center>
          <Stack align="center">
            <Title order={2}>Ошибка при загрузке постов</Title>
            <Text c="dimmed">Не удалось загрузить посты</Text>
          </Stack>
        </Center>
      </Container>
    );
  }

  const totalPages = pagination.total || 1;
  const queryParams: Record<string, string | number | undefined> = { view: "blogs" };
  if (search) queryParams.search = search;

  return (
    <Container size="xl" py="md">
      <Stack gap="md">
        <Group justify="space-between" align="center">
          <Title order={3}>Блоги</Title>
          <Button
            leftSection={<IconPlus size={16} />}
            onClick={handleCreatePost}
          >
            Создать пост
          </Button>
        </Group>

        <AdminBlogsSearchInput />

        {isLoading ? (
          <Stack gap="sm">
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
            <Skeleton height={35} radius="sm" />
          </Stack>
        ) : posts.length === 0 ? (
          <Center py="xl">
            <Stack align="center" gap="md">
              <Text c="dimmed">Посты не найдены</Text>
              <Button
                variant="light"
                leftSection={<IconPlus size={16} />}
                onClick={handleCreatePost}
              >
                Создать первый пост
              </Button>
            </Stack>
          </Center>
        ) : (
          <>
            <AdminBlogsTable
              posts={posts}
              onDeletePost={handleDeletePost}
              onEditPost={handleEditPost}
            />
            {totalPages > 1 && (
              <Stack align="center" gap="md">
                <NextPagination
                  pagination={{
                    page: page,
                    total: totalPages,
                  }}
                  baseUrl="/admin"
                  queryParams={queryParams}
                />
              </Stack>
            )}
          </>
        )}
      </Stack>

      <BlogPostForm
        opened={formOpened}
        onClose={() => {
          setFormOpened(false);
          setEditingPost(null);
        }}
        post={editingPost}
        onSubmit={handleFormSubmit}
      />

      <StatusMessage
        type={statusMessage?.type || "success"}
        message={statusMessage?.message || ""}
        opened={!!statusMessage}
        onClose={() => setStatusMessage(null)}
      />
    </Container>
  );
}



