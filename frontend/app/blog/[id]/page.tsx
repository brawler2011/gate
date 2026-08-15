import {Avatar, Container, Group, Stack, Text, Title} from "@mantine/core";
import ReactMarkdown from 'react-markdown';
import rehypeKatex from 'rehype-katex';
import remarkGfm from 'remark-gfm';
import remarkMath from 'remark-math';

import {DefaultLayout} from '@/components/shared';
import {publicApi} from "@/lib/api";
import {unwrapAndCache} from "@/lib/api2";
import {formatDate} from "@/lib/formatDate";

import 'katex/dist/katex.min.css';
import classes from "./styles.module.css";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const revalidate: number = 600; // 10 minutes

type Params = {
  id: string
};

export const generateStaticParams = async (): Promise<Params[]> => {
  const [error, data] = await publicApi.listPosts({page: 1, pageSize: 50});
  if (error) {
    return [];
  }

  return data.posts.map((post) => ({id: post.id}));
};

type Props = {
  params: Promise<Params>;
};

const getPost = unwrapAndCache(publicApi.getPostById);

export const generateMetadata = async ({params}: Props): Promise<Metadata> => {
  const {id} = await params;
  const post = await getPost({id});

  // TODO: SEO: opengraph, twitter, etc
  return {
    title: post.title,
    description: post.description,
  };
};

const Page = async ({params}: Props): Promise<ReactNode> => {
  const {id} = await params;
  const post = await getPost({id});

  // TODO: refactor jsx+css
  // BUG: margin-bottom: 0rem (1.25rem)
  // TODO: breadcrums
  // TODO: code syntax highlighting
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
                  name={post.author_username} 
                  size={48} 
                  radius="xl" 
                />
                <Stack gap={4}>
                  <Text size="lg" fw={600}>
                    {post.author_username}
                  </Text>
                  <Text size="sm" c="dimmed">
                    {formatDate(post.created_at)}
                  </Text>
                </Stack>
              </Group>
            </Stack>

            {/* Content */}
            <div className={classes.content}>
              <ReactMarkdown 
                remarkPlugins={[remarkGfm, remarkMath]}
                rehypePlugins={[rehypeKatex]}
              >
                {post.text}
              </ReactMarkdown>
            </div>
          </Stack>
        </article>
      </Container>
    </DefaultLayout>
  );
};

export default Page;
