import { Card, Text, Group, Avatar, Stack, Title, Skeleton, Image } from '@mantine/core';
import Link from 'next/link';
import classes from './styles.module.css';
import { formatDate } from '@/lib/formatDate';

export interface BlogPostProps {
  id: string;
  title: string;
  author: string;
  avatarUrl?: string;
  description: string;
  date?: string;
  previewImageUrl?: string;
}

export function BlogPost({ id, title, author, avatarUrl, description, date, previewImageUrl }: BlogPostProps) {
  return (
    <Card 
      component={Link} 
      href={`/blog/${id}`}
      shadow="sm" 
      padding={0} 
      radius="lg" 
      className={classes.card}
      style={{ textDecoration: 'none', color: 'inherit' }}
    >
      <Stack gap={0}>
        {previewImageUrl && (
          <div className={classes.imageContainer}>
            <Image
              src={previewImageUrl}
              alt={title}
              className={classes.previewImage}
              fallbackSrc="https://placehold.co/1200x500/e0e0e0/666?text=Blog+Post"
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
              <Text size="sm" fw={600} style={{ lineHeight: 1.2 }}>
                {author}
              </Text>
              {date && (
                <Text size="xs" c="dimmed" style={{ lineHeight: 1.2 }}>
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
  );
}

export function BlogPostSkeleton() {
  return (
    <Card shadow="sm" padding="lg" radius="md" withBorder>
      <Stack gap="md">
        <Stack gap="xs">
          <Group justify="space-between" align="start">
            <Skeleton height={28} width="70%" radius="sm" />
            <Skeleton height={24} width={100} radius="sm" />
          </Group>
          
          <Group gap="xs">
            <Skeleton height={26} width={26} circle />
            <Skeleton height={20} width={120} radius="sm" />
          </Group>
        </Stack>

        <Stack gap="xs">
          <Skeleton height={16} radius="sm" />
          <Skeleton height={16} radius="sm" />
          <Skeleton height={16} radius="sm" width="80%" />
        </Stack>
      </Stack>
    </Card>
  );
}

