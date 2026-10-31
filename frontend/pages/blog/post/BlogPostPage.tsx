import {Avatar, Container, Group, Stack, Text, Title} from "@mantine/core";
import ReactMarkdown from "react-markdown";
import {useLoaderData} from "react-router-dom";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

import "katex/dist/katex.min.css";

import {DefaultLayout} from "@/components/shared";
import {api, unwrap} from "@/lib/api";
import {formatDate} from "@/lib/lib";

import classes from "./styles.module.css";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams}: LoaderFunctionArgs) => {
  const {id} = routeParams as {id: string};
  const post = await unwrap(api.GET("/posts/{id}", {params: {path: {id}}}));
  return {post};
}) satisfies LoaderFunction;

const Page = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {post} = data;
  return (
    <DefaultLayout>
      <title>{post.title}</title>
      <Container size="md" py="xl">
        <article className={classes.article}>
          <Stack gap="xl">
            {/* Header */}
            <Stack gap="md">
              <Title order={1} className={classes.mainTitle}>
                {post.title}
              </Title>

              <Group gap="md">
                <Avatar name={post.author_username} size={48} radius="xl" />
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
              <ReactMarkdown remarkPlugins={[remarkGfm, remarkMath]} rehypePlugins={[rehypeKatex]}>
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
