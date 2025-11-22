import { DefaultLayout } from "@/components/Layout";
import { getBlogPost, getBlogPostIds } from "@/lib/blog";
import { formatDate } from "@/lib/formatDate";
import { Avatar, Container, Group, Stack, Text, Title } from "@mantine/core";
import { Metadata } from "next";
import { notFound } from "next/navigation";
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import classes from "./styles.module.css";

type Props = {
  params: Promise<{ id: string }>;
};

export async function generateStaticParams() {
  const ids = getBlogPostIds();
  return ids.map((id) => ({ id }));
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  const post = getBlogPost(id);

  if (!post) {
    return {
      title: "Пост не найден",
    };
  }

  return {
    title: post.title,
    description: post.description,
  };
}

export default async function BlogPostPage({ params }: Props) {
  const { id } = await params;
  const post = getBlogPost(id);

  if (!post) {
    notFound();
  }

  return (
    <DefaultLayout>
      <Container size="md" py="xl">
        <article className={classes.article}>
          <Stack gap="xl">
            {/* Header */}
            <Stack gap="md">
              <Title order={1} className={classes.mainTitle}>
                {post.title}
              </Title>

              <Group gap="md">
                <Avatar 
                  src={post.avatarUrl} 
                  name={post.author} 
                  size={48} 
                  radius="xl" 
                />
                <Stack gap={4}>
                  <Text size="lg" fw={600}>
                    {post.author}
                  </Text>
                  <Text size="sm" c="dimmed">
                    {formatDate(post.date)}
                  </Text>
                </Stack>
              </Group>
            </Stack>

            {/* Divider */}
            <div className={classes.divider} />

            {/* Content */}
            <div className={classes.content}>
              <ReactMarkdown remarkPlugins={[remarkGfm]}>
                {post.content}
              </ReactMarkdown>
            </div>
          </Stack>
        </article>
      </Container>
    </DefaultLayout>
  );
}

