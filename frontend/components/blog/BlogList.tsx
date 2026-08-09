"use client";

import {Stack, Text} from "@mantine/core";

import {BlogPost} from '@/components/blog/BlogPost';
import {NextPagination} from '@/components/shared/Pagination';

import type {PostModel, PaginationModel} from "@/contracts/core/v1";

type Props = {
  posts: PostModel[];
  pagination: PaginationModel;
};

export const BlogList = ({posts, pagination}: Props) => {
  if (posts.length === 0) { 
    return (
      <Text c="dimmed" ta="center" py="xl">
        Пока постов нет
      </Text>
    );
  }

  return (
    <Stack gap="md">
      {posts.map((post) => (
        <BlogPost
          key={post.id}
          id={post.id}
          title={post.title}
          author={post.author_username}
          date={post.created_at}
          description={post.description}
          previewImageUrl={post.preview_image_id}
        />
      ))}
      {pagination.total > 1 && (
        <Stack align="center" gap="md">
          <NextPagination
            pagination={pagination}
            baseUrl="/"
          />
        </Stack>
      )}
    </Stack>
  );
};
