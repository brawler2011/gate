import { DefaultLayout } from '@/components/shared';
import { isAuthenticated } from "@/lib/auth";
import { listPosts } from "@/lib/actions";
import type { PaginationModel } from "@contracts/gateway/v1";
import {
  Container,
  Group,
  Stack,
  Title,
} from "@mantine/core";
import { IconNews } from "@tabler/icons-react";
import { BlogList } from '@/components/blog/BlogList';

export const metadata = {
  title: "Главная",
};

type PageProps = {
  searchParams: Promise<{ page?: string }>;
};

export default async function Page({ searchParams }: PageProps) {
  const authenticated = await isAuthenticated();
  
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

  const renderBlogSection = () => (
    <Stack gap="md">
      <Group gap="xs">
        <IconNews size={28} color="var(--mantine-color-orange-6)" />
        <Title order={2}>Блог</Title>
      </Group>
      <BlogList 
        posts={blogPosts}
        pagination={pagination}
        error={!!error}
      />
    </Stack>
  );

  return (
    <DefaultLayout>
      <Container size="lg" py="xl">
        {authenticated ? (
          <Stack gap="md">
            {renderBlogSection()}
          </Stack>
        ) : (
          renderBlogSection()
        )}
      </Container>
    </DefaultLayout>
  );
}
