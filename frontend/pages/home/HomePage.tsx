import {Container, Group, Stack, Title, Grid, GridCol, Text} from "@mantine/core";
import {IconNews} from "@tabler/icons-react";

import {BlogList} from "@/components/blog/BlogList";
import {CompactBlogList} from "@/components/blog/CompactBlogList";
import {DashboardContestsList} from "@/components/contests/DashboardContestsList";
import {DashboardProblemsList} from "@/components/problems/DashboardProblemsList";
import {DefaultLayout} from "@/components/shared";
import {useSession} from "@/contexts/SessionContext";
import {api} from "@/lib/api";

import type {ReactNode} from "react";

const Page = (): ReactNode => {
  const {user, isAuthenticated: authenticated} = useSession();
  const dashboard = api.useQuery("get", "/users/me/dashboard", {}, {enabled: authenticated});
  const blog = api.useQuery("get", "/posts", {params: {query: {page: 1, pageSize: 5}}});
  const dashboardData = dashboard.data;
  const dashboardError = dashboard.error;
  const blogData = blog.data;
  const blogError = blog.error;

  if (authenticated) {
    // Authenticated user view: Dashboard with personal quick navigation & compact sidebar blog
    const recentContests = dashboardData?.recent_contests || [];
    const myProblems = dashboardData?.my_problems || [];
    const blogPosts = blogData?.posts || [];

    return (
      <DefaultLayout headerUser={user}>
        <title>Главная</title>
        <Container size="lg" py="xl">
          <Grid gutter="xl">
            <GridCol span={{base: 12, md: 8}}>
              <Stack gap="xl">
                <Stack gap="sm">
                  <Title order={2} size="h4" fw={700}>
                    Недавние контесты
                  </Title>
                  {dashboardError ? (
                    <Text size="sm" c="red">
                      Не удалось загрузить данные контестов
                    </Text>
                  ) : (
                    <DashboardContestsList contests={recentContests} />
                  )}
                </Stack>

                <Stack gap="sm" mt="md">
                  <Title order={2} size="h4" fw={700}>
                    Ваши задачи
                  </Title>
                  {dashboardError ? (
                    <Text size="sm" c="red">
                      Не удалось загрузить ваши задачи
                    </Text>
                  ) : (
                    <DashboardProblemsList problems={myProblems} />
                  )}
                </Stack>
              </Stack>
            </GridCol>

            <GridCol span={{base: 12, md: 4}}>
              <CompactBlogList posts={blogPosts} error={!!blogError} />
            </GridCol>
          </Grid>
        </Container>
      </DefaultLayout>
    );
  }

  const blogPosts = blogData?.posts || [];

  return (
    <DefaultLayout headerUser={null}>
      <title>Главная</title>
      <Container size="md" py="xl">
        <Stack gap="xl">
          <Stack gap="md">
            <Group gap="xs">
              <IconNews size={28} color="var(--mantine-color-orange-6)" />
              <Title order={2} size="h3" style={{fontWeight: 700}}>
                Блог
              </Title>
            </Group>
            <BlogList posts={blogPosts} pagination={{total: 1, page: 1}} />
          </Stack>
        </Stack>
      </Container>
    </DefaultLayout>
  );
};

export default Page;
