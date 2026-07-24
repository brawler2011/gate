"use client";

import { Stack, Text, Card, Title, Button, Group, Box } from "@mantine/core";
import { IconChevronRight, IconNews } from "@tabler/icons-react";
import Link from "next/link";
import { formatDate } from "@/lib/formatDate";
import type { PostModel } from "@contracts/core/v1";

type CompactBlogListProps = {
  posts: PostModel[];
  error?: boolean;
};

export function CompactBlogList({ posts, error }: CompactBlogListProps) {
  if (error) {
    return <Text size="sm" c="red">Не удалось загрузить посты</Text>;
  }

  if (posts.length === 0) {
    return null;
  }

  return (
    <Card 
      withBorder 
      shadow="sm" 
      radius="md" 
      p="md"
      style={{
        backgroundColor: "var(--mantine-color-body)",
        display: "flex",
        flexDirection: "column",
        height: "100%",
      }}
    >
      <Stack gap="sm" style={{ flex: 1 }}>
        <Group gap="xs" mb="xs">
          <IconNews size={20} color="var(--mantine-color-blue-filled)" />
          <Title order={3} size="h4" style={{ fontWeight: 700 }}>
            Блог
          </Title>
        </Group>

        <Stack gap="md">
          {posts.map((post) => (
            <Box 
              key={post.id}
              component={Link}
              href={`/blog/${post.id}`}
              style={{
                textDecoration: "none",
                color: "inherit",
                display: "block",
              }}
            >
              <Stack gap={4} style={{ borderBottom: "1px solid var(--mantine-color-default-border)", paddingBottom: "12px" }}>
                <Title 
                  order={4} 
                  size="sm" 
                  fw={600}
                  style={{
                    transition: "color 0.2s ease",
                    cursor: "pointer",
                  }}
                  onMouseEnter={(e) => {
                    (e.target as HTMLElement).style.color = "var(--mantine-color-blue-filled)";
                  }}
                  onMouseLeave={(e) => {
                    (e.target as HTMLElement).style.color = "";
                  }}
                >
                  {post.title || "Без названия"}
                </Title>
                <Group gap="xs">
                  <Text size="xs" c="dimmed" fw={500}>
                    {post.author_username || "Аноним"}
                  </Text>
                  <Text size="xs" c="dimmed">•</Text>
                  <Text size="xs" c="dimmed">
                    {post.created_at ? formatDate(post.created_at) : ""}
                  </Text>
                </Group>
                {post.description && (
                  <Text size="xs" c="dimmed" lineClamp={2}>
                    {post.description}
                  </Text>
                )}
              </Stack>
            </Box>
          ))}
        </Stack>
      </Stack>

      <Button
        component={Link}
        href="/blog"
        variant="light"
        color="blue"
        size="xs"
        radius="md"
        mt="md"
        rightSection={<IconChevronRight size={14} />}
        fullWidth
      >
        Показать все
      </Button>
    </Card>
  );
}
