"use client";

import { Stack, Text } from "@mantine/core";
import { BlogPost } from '@/components/blog/BlogPost';
import { NextPagination } from '@/components/shared/Pagination';
import type { PostModel, PaginationModel } from "@contracts/gateway/v1";

type BlogListProps = {
  posts: PostModel[];
  pagination: PaginationModel;
  error?: boolean;
};

export function BlogList({ posts, pagination, error }: BlogListProps) {
  if (error) {
    return <Text c="dimmed">Не удалось загрузить посты</Text>;
  }

  if (posts.length === 0) {
    return <Text c="dimmed">Пока нет постов</Text>;
  }

  return (
    <Stack gap="md">
      {posts.map((post) => (
        <BlogPost
          key={post.id}
          id={post.id || ""}
          title={post.title || "Без названия"}
          author={post.author_username || "Аноним"}
          date={post.created_at}
          description={post.description || ""}
          previewImageUrl={post.preview_image_id}
        />
      ))}
      {pagination.total > 1 && (
        <Stack align="center" gap="md">
          <NextPagination
            pagination={{
              page: pagination.page,
              total: pagination.total,
            }}
            baseUrl="/"
            queryParams={{}}
          />
        </Stack>
      )}
    </Stack>
  );
}
