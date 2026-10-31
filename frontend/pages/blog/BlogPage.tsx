import {Container, Group, Stack, Title} from "@mantine/core";
import {IconNews} from "@tabler/icons-react";
import {useSearchParams, Navigate} from "react-router-dom";

import {BlogList} from "@/components/blog";
import {DefaultLayout} from "@/components/shared";
import {QueryState} from "@/components/shared/QueryState";
import {api} from "@/lib/api";
import {parsePage} from "@/lib/lib";

import type {ReactNode} from "react";

const Page = (): ReactNode => {
  const [search] = useSearchParams();
  const searchParams = Object.fromEntries(search);
  const page = parsePage(searchParams.page);
  const routeQuery = api.useQuery(
    "get",
    "/posts",
    {params: {query: {page: page ?? 1, pageSize: 5}}},
    {enabled: Boolean(page)},
  );
  if (!page) {
    return <Navigate to="/blog" replace />;
  }
  if (!routeQuery.data) {
    return (
      <>
        <title>Блог</title>
        <QueryState error={routeQuery.error} />
      </>
    );
  }
  const data = routeQuery.data;
  return (
    <DefaultLayout>
      <title>Блог</title>
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
