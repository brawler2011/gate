import {Avatar, Container, Group, Stack, Text, Title} from "@mantine/core";
import ReactMarkdown from "react-markdown";
import rehypeKatex from "rehype-katex";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

import "katex/dist/katex.min.css";

import {DefaultLayout} from "@/components/shared";
import {api, unwrap} from "@/lib/api";
import {formatDate} from "@/lib/lib";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";

import classes from "./styles.module.css";

import type {ReactNode} from "react";

const loadPage = async ({params}: Props) => {
  const {id} = params;
  const post = await getPost(id);
  return {id, post};
};

type Params = {
  id: string;
};

type Props = {
  params: Params;
};

const getPost = unwrap(api.getPostById);

const Page = ({params}: Props): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/blog/[id]/page.tsx", params], () => loadPage({params}));
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {post} = routeQuery.data;
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
