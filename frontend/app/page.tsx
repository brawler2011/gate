import { DefaultLayout } from '@/components/shared';
import { isAuthenticated } from "@/lib/auth";
import { listPosts, getMyDashboard } from "@/lib/actions";
import type { PaginationModel } from "@contracts/core/v1";
import {
  Container,
  Group,
  Stack,
  Title,
  Grid,
  GridCol,
  Text,
} from "@mantine/core";
import { IconNews } from "@tabler/icons-react";
import { BlogList } from '@/components/blog/BlogList';
import { CompactBlogList } from '@/components/blog/CompactBlogList';
import { DashboardContestsList } from '@/components/contests/DashboardContestsList';
import { DashboardProblemsList } from '@/components/problems/DashboardProblemsList';

export const metadata = {
  title: "Главная",
};

type PageProps = {
  searchParams: Promise<{ page?: string }>;
};

export default async function Page({ searchParams }: PageProps) {
  const authenticated = await isAuthenticated();
  const params = await searchParams;
  const currentPage = parseInt(params?.page || "1", 10) || 1;

  if (authenticated) {
    // Authenticated user view: Dashboard with personal quick navigation & compact sidebar blog
    const [
      [dashboardError, dashboardData],
      [blogError, blogData]
    ] = await Promise.all([
      getMyDashboard(),
      listPosts(1, 5) // Fetch top 5 blog posts for the sidebar
    ]);

    const recentContests = dashboardData?.recent_contests || [];
    const myProblems = dashboardData?.my_problems || [];
    const blogPosts = blogData?.posts || [];

    return (
      <DefaultLayout>
        <Container size="lg" py="xl">
          <Grid gutter="xl">
            <GridCol span={{ base: 12, md: 8 }}>
              <Stack gap="xl">


                <Stack gap="sm">
                  <Title order={2} size="h4" fw={700}>
                    Недавние контесты
                  </Title>
                  {dashboardError ? (
                    <Text size="sm" c="red">Не удалось загрузить данные контестов</Text>
                  ) : (
                    <DashboardContestsList contests={recentContests} />
                  )}
                </Stack>

                <Stack gap="sm" mt="md">
                  <Title order={2} size="h4" fw={700}>
                    Ваши задачи
                  </Title>
                  {dashboardError ? (
                    <Text size="sm" c="red">Не удалось загрузить ваши задачи</Text>
                  ) : (
                    <DashboardProblemsList problems={myProblems} />
                  )}
                </Stack>
              </Stack>
            </GridCol>

            <GridCol span={{ base: 12, md: 4 }}>
              <CompactBlogList posts={blogPosts} error={!!blogError} />
            </GridCol>
          </Grid>
        </Container>
      </DefaultLayout>
    );
  }

  // Guest view: Welcome banner & full blog feed
  const [blogError, blogData] = await listPosts(currentPage, 5);
  const blogPosts = blogData?.posts || [];
  const pagination: PaginationModel = {
    total: blogData?.pagination?.total ?? 0,
    page: blogData?.pagination?.page ?? currentPage,
  };

  return (
    <DefaultLayout>
      <Container size="md" py="xl">
        <Stack gap="xl">
          <Stack gap="md">
            <Group gap="xs">
              <IconNews size={28} color="var(--mantine-color-orange-6)" />
              <Title order={2} size="h3" style={{ fontWeight: 700 }}>Блог</Title>
            </Group>
            <BlogList 
              posts={blogPosts}
              pagination={pagination}
              error={!!blogError}
            />
          </Stack>
        </Stack>
      </Container>
    </DefaultLayout>
  );
}
