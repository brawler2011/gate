"use client";
import {
  Avatar,
  Card,
  Group,
  Image,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import NextImage from "next/image";
import Link from "next/link";

import {formatDate} from "@/lib/formatDate";

import classes from "./BlogPost.module.css";

export interface BlogPostProps {
  id: string;
  title: string;
  author: string;
  avatarUrl?: string;
  previewImageUrl?: string;
  description: string;
  date?: string;
}

export const BlogPost = ({
  id,
  title,
  author,
  avatarUrl,
  previewImageUrl,
  description,
  date,
}: BlogPostProps) => {
  const imageUrl = previewImageUrl ? `/api/posts/${id}/image` : null;

  return (
    <Link
      href={`/blog/${id}`}
      style={{textDecoration: "none", color: "inherit", display: "block"}}
    >
      <Card shadow="sm" padding={0} radius="lg" className={classes.card}>
        <Stack gap={0}>
          {imageUrl && (
            <div className={classes.imageContainer}>
              <Image
                component={NextImage}
                src={imageUrl}
                alt={title}
                fill
                sizes="(max-width: 768px) 100vw, 100vw"
                className={classes.previewImage}
              />
            </div>
          )}

          <Stack gap="md" p="xl">
            <Title order={3} size="h3" className={classes.title}>
              {title}
            </Title>

            <Group gap="xs">
              <Avatar src={avatarUrl} name={author} size={32} radius="xl" />
              <Stack gap={2}>
                <Text size="sm" fw={600} style={{lineHeight: 1.2}}>
                  {author}
                </Text>
                {date && (
                  <Text size="xs" c="dimmed" style={{lineHeight: 1.2}}>
                    {formatDate(date)}
                  </Text>
                )}
              </Stack>
            </Group>

            <Text className={classes.description} lineClamp={3}>
              {description}
            </Text>
          </Stack>
        </Stack>
      </Card>
    </Link>
  );
};
