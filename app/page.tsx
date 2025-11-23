import { DefaultLayout } from "@/components/Layout";
import { getOrySession } from "@/lib/api";
import {
  Box,
  Button,
  Card,
  Container,
  Group,
  SimpleGrid,
  Stack,
  Text,
  Title,
} from "@mantine/core";
import { IconPuzzle, IconTrophy, IconNews } from "@tabler/icons-react";
import Link from "next/link";
import { HomeContestsList } from "@/components/HomeContestsList";
import { HomeProblemsTable } from "@/components/HomeProblemsTable";
import { BlogPost } from "@/components/BlogPost/BlogPost";
import { APP_COLORS } from "@/lib/theme/colors";
import type { ContestModel } from "../../contracts/core/v1";
import { getAllBlogPosts } from "@/lib/blog";

export const metadata = {
  title: "Главная",
};


// Mock data for problems
const mockProblems = [
  {
    id: "1",
    contest_id: "mock-contest-1",
    problem_id: "mock-prob-1",
    position: 1,
    title: "Сумма чисел",
    time_limit: 1000,
    memory_limit: 256,
  },
  {
    id: "2",
    contest_id: "mock-contest-1",
    problem_id: "mock-prob-2",
    position: 2,
    title: "Поиск в глубину",
    time_limit: 2000,
    memory_limit: 512,
  },
  {
    id: "3",
    contest_id: "mock-contest-2",
    problem_id: "mock-prob-3",
    position: 3,
    title: "Динамическое программирование",
    time_limit: 3000,
    memory_limit: 256,
  },
  {
    id: "4",
    contest_id: "mock-contest-2",
    problem_id: "mock-prob-4",
    position: 4,
    title: "Сортировка подсчетом",
    time_limit: 1500,
    memory_limit: 128,
  },
  {
    id: "5",
    contest_id: "mock-contest-3",
    problem_id: "mock-prob-5",
    position: 5,
    title: "Кратчайший путь",
    time_limit: 2500,
    memory_limit: 512,
  },
];

// Mock data for contests
const mockContests: ContestModel[] = [
  {
    id: "mock-contest-1",
    title: "Тренировочный раунд #1",
    description: "Тренировочный контест для начинающих",
    visibility: "public",
    monitor_scope: "public",
    submissions_list_scope: "public",
    submissions_review_scope: "public",
    created_by: "system",
    created_at: new Date("2024-11-01").toISOString(),
    updated_at: new Date("2024-11-01").toISOString(),
  },
  {
    id: "mock-contest-2",
    title: "Олимпиада по алгоритмам",
    description: "Ежегодная олимпиада по алгоритмам",
    visibility: "public",
    monitor_scope: "public",
    submissions_list_scope: "public",
    submissions_review_scope: "public",
    created_by: "system",
    created_at: new Date("2024-11-05").toISOString(),
    updated_at: new Date("2024-11-05").toISOString(),
  },
  {
    id: "mock-contest-3",
    title: "Еженедельный контест",
    description: "Регулярный еженедельный контест",
    visibility: "public",
    monitor_scope: "public",
    submissions_list_scope: "public",
    submissions_review_scope: "public",
    created_by: "system",
    created_at: new Date("2024-11-10").toISOString(),
    updated_at: new Date("2024-11-10").toISOString(),
  },
  {
    id: "mock-contest-4",
    title: "Динамическое программирование",
    description: "Задачи на динамическое программирование",
    visibility: "public",
    monitor_scope: "public",
    submissions_list_scope: "public",
    submissions_review_scope: "public",
    created_by: "system",
    created_at: new Date("2024-11-12").toISOString(),
    updated_at: new Date("2024-11-12").toISOString(),
  },
  {
    id: "mock-contest-5",
    title: "Графы и деревья",
    description: "Задачи на графы и деревья",
    visibility: "public",
    monitor_scope: "public",
    submissions_list_scope: "public",
    submissions_review_scope: "public",
    created_by: "system",
    created_at: new Date("2024-11-14").toISOString(),
    updated_at: new Date("2024-11-14").toISOString(),
  },
];

export default async function Page() {
  const session = await getOrySession();
  const blogPosts = getAllBlogPosts();
  const isAuthenticated = !!session;

  return (
    <DefaultLayout>
      <Container size="lg" py="xl">
        {isAuthenticated ? (
          <Stack gap="md">
            {/* Two columns for Problems and Contests */}
            <SimpleGrid cols={{ base: 1, md: 2 }} spacing="md">
              {/* Problems Section */}
              <Card shadow="sm" padding="lg" radius="md" withBorder h="100%">
                <Stack gap="md" h="100%">
                  <Group justify="space-between" align="center">
                    <Group gap="xs">
                      <IconPuzzle size={28} color={`var(--mantine-color-${APP_COLORS.problems}-6)`} />
                      <Title order={2}>Продолжить решение</Title>
                    </Group>
                    <Button
                      component={Link}
                      href="/workshop?view=problems"
                      variant="subtle"
                      color={APP_COLORS.problems}
                    >
                      Все задачи
                    </Button>
                  </Group>
                  
                  <Box flex={1}>
                    <HomeProblemsTable problems={mockProblems} />
                  </Box>
                </Stack>
              </Card>

              {/* Contests Section */}
              <Card shadow="sm" padding="lg" radius="md" withBorder h="100%">
                <Stack gap="md" h="100%">
                  <Group justify="space-between" align="center">
                    <Group gap="xs">
                      <IconTrophy size={28} color={`var(--mantine-color-${APP_COLORS.contests}-6)`} />
                      <Title order={2}>Мои контесты</Title>
                    </Group>
                    <Button
                      component={Link}
                      href="/workshop?view=contests"
                      variant="subtle"
                      color={APP_COLORS.contests}
                    >
                      Все контесты
                    </Button>
                  </Group>
                  
                  <Box flex={1}>
                    <HomeContestsList contests={mockContests} />
                  </Box>
                </Stack>
              </Card>
            </SimpleGrid>

            {/* Blog Section - Full width below */}
            <Stack gap="md">
              <Group gap="xs">
                <IconNews size={28} color="var(--mantine-color-orange-6)" />
                <Title order={2}>Блог</Title>
              </Group>
              <Stack gap="md">
                {blogPosts.map((post) => (
                  <BlogPost
                    key={post.id}
                    id={post.id}
                    title={post.title}
                    author={post.author}
                    avatarUrl={post.avatarUrl}
                    date={post.date}
                    description={post.description}
                    previewImageUrl={post.previewImageUrl}
                  />
                ))}
              </Stack>
            </Stack>
          </Stack>
        ) : (
          /* For non-authenticated users - only show Blog */
          <Stack gap="md">
            <Group gap="xs">
              <IconNews size={28} color="var(--mantine-color-orange-6)" />
              <Title order={2}>Блог</Title>
            </Group>
            <Stack gap="md">
              {blogPosts.map((post) => (
                <BlogPost
                  key={post.id}
                  id={post.id}
                  title={post.title}
                  author={post.author}
                  avatarUrl={post.avatarUrl}
                  date={post.date}
                  description={post.description}
                  previewImageUrl={post.previewImageUrl}
                />
              ))}
            </Stack>
          </Stack>
        )}
      </Container>
    </DefaultLayout>
  );
}

