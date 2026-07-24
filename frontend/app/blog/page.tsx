import { DefaultLayout } from '@/components/shared';
import { listPosts } from "@/lib/actions";
import type { PaginationModel } from "@contracts/core/v1";
import { Container, Group, Stack, Title } from "@mantine/core";
import { IconNews } from "@tabler/icons-react";
import { BlogList } from '@/components/blog/BlogList';

export const metadata = {
  title: "Блог",
};

type PageProps = {
  searchParams: Promise<{ page?: string }>;
};

export default async function BlogPage({ searchParams }: PageProps) {
  // Get current page from query params, default to 1
  const params = await searchParams;
  const currentPage = parseInt(params?.page || "1", 10) || 1;
  
  // Fetch blog posts from API with pagination (5 posts per page)
  const [error, postsData] = await listPosts(currentPage, 5);
  const blogPosts = postsData?.posts || [];
  const pagination: PaginationModel = {
    total: postsData?.pagination?.total ?? 0,
    page: postsData?.pagination?.page ?? currentPage,
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
}
