import {Container, Group, Stack, Title} from "@mantine/core";
import {IconNews} from "@tabler/icons-react";
import {redirect} from "next/navigation";

import {BlogList} from '@/components/blog';
import {DefaultLayout} from '@/components/shared';
import {api, unwrap} from "@/lib/api";
import {parsePage} from "@/lib/lib2";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Блог",
};

type Props = {
  searchParams: Promise<{ page?: string }>;
};

const Page = async ({searchParams}: Props): Promise<ReactNode> => {
  const params = await searchParams;
  const page = parsePage(params.page);
  if (!page) {
    redirect("/blog");
  }

  // FIXME: page=109348 => error=null, posts=[]
  const data = await unwrap(api.listPosts)({page, pageSize: 5});

  return (
    <DefaultLayout>
      <Container size="md" py="xl">
        <Stack gap="xl">
          <Group gap="xs">
            <IconNews size={32} color="var(--mantine-color-orange-6)" />
            <Title order={1}>Блог</Title>
          </Group>
          <BlogList
            posts={data.posts}
            pagination={data.pagination}
          />
        </Stack>
      </Container>
    </DefaultLayout>
  );
};

export default Page;
