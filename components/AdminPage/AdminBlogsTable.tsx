"use client";

import {
  ActionIcon,
  Avatar,
  Badge,
  Box,
  Group,
  Table,
  Text,
} from "@mantine/core";
import { IconEdit, IconTrash } from "@tabler/icons-react";
import { useRouter } from "next/navigation";
import { useState } from "react";
import type { PostModel } from "@contracts/gateway/v1";
import { TruncatedWithCopy } from "../TruncatedWithCopy";
import { DeleteBlogPostModal } from "./DeleteBlogPostModal";
import { formatDate } from "@/lib/formatDate";
import classes from "./styles.module.css";

type AdminBlogsTableProps = {
  posts: PostModel[];
  onDeletePost: (postId: string) => Promise<void>;
  onEditPost: (post: PostModel) => void;
};

export function AdminBlogsTable({ posts, onDeletePost, onEditPost }: AdminBlogsTableProps) {
  const router = useRouter();
  const [deleteModalOpened, setDeleteModalOpened] = useState(false);
  const [postToDelete, setPostToDelete] = useState<PostModel | null>(null);
  const [deletingId, setDeletingId] = useState<string | null>(null);

  const handleRowClick = (postId: string) => {
    router.push(`/blog/${postId}`);
  };

  const handleEditClick = (e: React.MouseEvent, post: PostModel) => {
    e.stopPropagation();
    onEditPost(post);
  };

  const handleAuthorClick = (e: React.MouseEvent, authorId: string | undefined) => {
    e.stopPropagation();
    if (authorId) {
      router.push(`/users/${authorId}`);
    }
  };

  const handleDeleteClick = (e: React.MouseEvent, post: PostModel) => {
    e.stopPropagation();
    setPostToDelete(post);
    setDeleteModalOpened(true);
  };

  const handleDeleteConfirm = async () => {
    if (!postToDelete) return;
    
    setDeletingId(postToDelete.id || "");
    try {
      await onDeletePost(postToDelete.id || "");
    } finally {
      setDeletingId(null);
      setPostToDelete(null);
    }
  };

  return (
    <>
      <Box className={classes.tableContainer}>
        <Table className={classes.table} verticalSpacing="xs">
          <Table.Thead className={classes.thead}>
            <Table.Tr>
              <Table.Th style={{ width: "5%" }}></Table.Th>
              <Table.Th style={{ width: "30%" }}>Название</Table.Th>
              <Table.Th style={{ width: "12%" }}>ID</Table.Th>
              <Table.Th style={{ width: "15%" }}>Автор</Table.Th>
              <Table.Th style={{ width: "12%" }}>Создан</Table.Th>
              <Table.Th style={{ width: "12%" }}>Обновлён</Table.Th>
              <Table.Th style={{ width: "10%" }}>Действия</Table.Th>
            </Table.Tr>
          </Table.Thead>
          <Table.Tbody className={classes.tbody}>
            {posts.map((post) => {
              return (
                <Table.Tr
                  key={post.id}
                  onClick={() => handleRowClick(post.id || "")}
                >
                  <Table.Td>
                    <Avatar
                      src={post.preview_image_id ? `/api/blogs/posts/${post.preview_image_id}/image` : undefined}
                      size={32}
                      radius="sm"
                    />
                  </Table.Td>
                  <Table.Td>
                    <Text className={classes.titleCell} lineClamp={1}>
                      {post.title}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <TruncatedWithCopy value={post.id || ""} />
                  </Table.Td>
                  <Table.Td>
                    <Badge
                      variant="light"
                      color="blue"
                      tt="none"
                      size="sm"
                      className={classes.authorBadge}
                      onClick={(e) => handleAuthorClick(e, post.author_id)}
                    >
                      {post.author_username || post.author_id?.slice(0, 8) || "—"}
                    </Badge>
                  </Table.Td>
                  <Table.Td>
                    <Text className={classes.dateCell}>
                      {formatDate(post.created_at)}
                    </Text>
                  </Table.Td>
                  <Table.Td>
                    <Text className={classes.dateCell}>
                      {formatDate(post.updated_at)}
                    </Text>
                  </Table.Td>
                  <Table.Td className={classes.actionsCell}>
                    <Group gap="xs" wrap="nowrap">
                      <ActionIcon
                        color="blue"
                        variant="subtle"
                        onClick={(e) => handleEditClick(e, post)}
                      >
                        <IconEdit size={16} />
                      </ActionIcon>
                      <ActionIcon
                        color="red"
                        variant="subtle"
                        onClick={(e) => handleDeleteClick(e, post)}
                        loading={deletingId === post.id}
                      >
                        <IconTrash size={16} />
                      </ActionIcon>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              );
            })}
          </Table.Tbody>
        </Table>
      </Box>

      {postToDelete && (
        <DeleteBlogPostModal
          opened={deleteModalOpened}
          onClose={() => {
            setDeleteModalOpened(false);
            setPostToDelete(null);
          }}
          post={{
            id: postToDelete.id || "",
            title: postToDelete.title || "Без названия",
          }}
          onSubmit={handleDeleteConfirm}
        />
      )}
    </>
  );
}




