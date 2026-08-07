import { Container, Group, Stack, Title } from "@mantine/core";
import { IconNews } from "@tabler/icons-react";
import { redirect } from "next/navigation";

import { BlogList } from '@/components/blog/BlogList';
import { DefaultLayout } from '@/components/shared';
import { api } from "@/lib/api";
import { parsePage } from "@/lib/lib2";

import type { PaginationModel } from "@/contracts/core/v1";
import type { Metadata } from "next";


export const metadata: Metadata = {
  title: "Блог",
};

type Props = {
  searchParams: Promise<{ page?: string }>;
};

const Page = async ({ searchParams }: Props) => {
  const params = await searchParams;
  const page = parsePage(params.page);
  if (!page) {
    redirect("/blog");
  }

  const [error, data] = await api.listPosts({ page, pageSize: 5 });
  if (error) {
    throw error;
  }

  const blogPosts = data.posts || [];
  const pagination: PaginationModel = {
    total: data.pagination?.total ?? 0,
    page: data.pagination?.page ?? page,
  };

  return (
    <DefaultLayout>
      <Container size="md" py="xl">
        <Stack gap="xl">
          <Group gap="xs">
            <IconNews size={32} color="var(--mantine-color-orange-6)" />
            <Title order={1}>Блог</Title>
          </Group>

          <BlogList
            posts={blogPosts}
            pagination={pagination}
            error={!!error}
          />
        </Stack>
      </Container>
    </DefaultLayout>
  );
};

export default Page;
