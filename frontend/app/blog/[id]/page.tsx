import { DefaultLayout } from '@/components/shared';
import { getPostByIdPublic, listPostsPublic } from "@/lib/actions";
import { formatDate } from "@/lib/formatDate";
import { Avatar, Container, Group, Stack, Text, Title } from "@mantine/core";
import { Metadata } from "next";
import { notFound } from "next/navigation";
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';
import rehypeKatex from 'rehype-katex';
import 'katex/dist/katex.min.css';
import classes from "./styles.module.css";

// Revalidate cache every 10 minutes
export const revalidate = 600;

type Props = {
  params: Promise<{ id: string }>;
};

export async function generateStaticParams() {
  try {
    const [error, postsData] = await listPostsPublic(1, 50);
    if (error || !postsData || !postsData.posts) return [];
    return postsData.posts
      .filter((post) => post.id !== undefined && post.id !== null)
      .map((post) => ({
        id: post.id!.toString(),
      }));
  } catch (e) {
    console.warn("Бэкенд недоступен при сборке, статический рендеринг пропущен:", e);
    return [];
  }
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { id } = await params;
  const [error, post] = await getPostByIdPublic(id);

  if (error || !post) {
    return {
      title: "Пост не найден",
    };
  }

  return {
    title: post.title || "Пост",
    description: post.description || "",
  };
}

export default async function BlogPostPage({ params }: Props) {
  const { id } = await params;
  const [error, post] = await getPostByIdPublic(id);

  if (error || !post) {
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
                  name={post.author_username || "Аноним"} 
                  size={48} 
                  radius="xl" 
                />
                <Stack gap={4}>
                  <Text size="lg" fw={600}>
                    {post.author_username || "Аноним"}
                  </Text>
                  {post.created_at && (
                    <Text size="sm" c="dimmed">
                      {formatDate(post.created_at)}
                    </Text>
                  )}
                </Stack>
              </Group>
            </Stack>

            {/* Content */}
            <div className={classes.content}>
              <ReactMarkdown 
                remarkPlugins={[remarkGfm, remarkMath]}
                rehypePlugins={[rehypeKatex]}
              >
                {post.text || ""}
              </ReactMarkdown>
            </div>
          </Stack>
        </article>
      </Container>
    </DefaultLayout>
  );
}
