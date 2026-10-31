import {Container, Group, Stack, Title} from "@mantine/core";
import {IconNews} from "@tabler/icons-react";
import {Navigate} from "react-router-dom";

import {BlogList} from "@/components/blog";
import {DefaultLayout} from "@/components/shared";
import {useListPosts} from "@/contracts/core/v1/api";
import {parsePage} from "@/lib/lib";
import {PageQueryState} from "@/lib/pageQuery";

import type {ReactNode} from "react";

type Props = {
  searchParams: { page?: string };
};

const Page = ({searchParams}: Props): ReactNode => {
  const page = parsePage(searchParams.page);
  const routeQuery = useListPosts({page: page ?? 1, pageSize: 5}, {query: {enabled: Boolean(page)}});
  if (!page) {
    return <Navigate to="/blog" replace />;
  }
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const data = routeQuery.data;
  return (
    <DefaultLayout>
      <Container size="md" py="xl">
        <Stack gap="xl">
          <Group gap="xs">
            <IconNews size={32} color="var(--mantine-color-orange-6)" />
            <Title order={1}>Блог</Title>
          </Group>
          <BlogList posts={data.posts} pagination={data.pagination} />
        </Stack>
      </Container>
    </DefaultLayout>
  );
};

export default Page;
