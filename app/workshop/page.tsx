import { DefaultLayout } from "@/components/Layout";
import { WorkshopContestsContentSkeleton } from "@/components/WorkshopPage/WorkshopContestsContentSkeleton";
import { WorkshopContestsWrapper } from "@/components/WorkshopPage/WorkshopContestsWrapper";
import { WorkshopHeader } from "@/components/WorkshopPage/WorkshopHeader";
import { WorkshopPageWrapper } from "@/components/WorkshopPage/WorkshopPageWrapper";
import { WorkshopProblemsContentSkeleton } from "@/components/WorkshopPage/WorkshopProblemsContentSkeleton";
import { WorkshopProblemsWrapper } from "@/components/WorkshopPage/WorkshopProblemsWrapper";
import { WorkshopTabs } from "@/components/WorkshopPage/WorkshopTabs";
import { getContests, getProblems } from "@/lib/actions";
import { getSession } from "@/lib/auth";
import { Alert, Center, Container, Stack } from "@mantine/core";
import { IconAlertCircle } from "@tabler/icons-react";
import { Metadata } from "next";
import { Suspense } from "react";
export const metadata: Metadata = {
  title: "Мастерская",
  description: "",
};

type Props = {
  searchParams: Promise<{ page?: string; view?: string; search?: string }>;
};

const ProblemsView = async ({
  page,
  search,
  isAuthenticated,
}: {
  page: number;
  search?: string;
  isAuthenticated: boolean;
}) => {
  const problemsData = await getProblems(page, 20, undefined, undefined, true);

  if (!problemsData) {
    return (
      <Center py="xl">
        <Stack align="center">
          <Alert
            icon={<IconAlertCircle size="1rem" />}
            title="Ошибка загрузки"
            color="red"
          >
            Не удалось загрузить список задач. Попробуйте обновить страницу.
          </Alert>
        </Stack>
      </Center>
    );
  }

  return (
    <WorkshopProblemsWrapper
      problems={problemsData.problems}
      pagination={problemsData.pagination}
      isAuthenticated={isAuthenticated}
      owner="me"
    />
  );
};

const ContestsView = async ({
  page,
  search,
}: {
  page: number;
  search?: string;
}) => {
  const contestsData = await getContests(page, 10, search);

  if (!contestsData) {
    return (
      <Center py="xl">
        <Stack align="center">
          <Alert
            icon={<IconAlertCircle size="1rem" />}
            title="Ошибка загрузки"
            color="red"
          >
            Не удалось загрузить список контестов. Попробуйте обновить страницу.
          </Alert>
        </Stack>
      </Center>
    );
  }

  return (
    <WorkshopContestsWrapper
      contests={contestsData.contests}
      pagination={contestsData.pagination}
    />
  );
};

const WorshopPageContent = async ({
  page,
  view,
  search,
}: {
  page: number;
  view: string;
  search?: string;
}) => {
  const session = await getSession();
  const isAuthenticated = !!session?.active;

  return (
    <WorkshopPageWrapper>
      <Stack gap="md">
        <WorkshopHeader isAuthenticated={isAuthenticated} />
        <WorkshopTabs isAuthenticated={isAuthenticated} />
        {view === "problems" ? (
          <Suspense fallback={<WorkshopProblemsContentSkeleton />}>
            <ProblemsView
              page={page}
              search={search}
              isAuthenticated={isAuthenticated}
            />
          </Suspense>
        ) : (
          <Suspense fallback={<WorkshopContestsContentSkeleton />}>
            <ContestsView
              page={page}
              search={search}
            />
          </Suspense>
        )}
      </Stack>
    </WorkshopPageWrapper>
  );
};

const Page = async (props: Props) => {
  const resolvedSearchParams = await props.searchParams;
  const page = Number(resolvedSearchParams.page) || 1;
  const view = resolvedSearchParams.view || "contests";
  const search = resolvedSearchParams.search;

  // Fetch user data on server

  return (
    <DefaultLayout>
      <Container size="lg" py="lg">
        <WorshopPageContent page={page} view={view} search={search} />
      </Container>
    </DefaultLayout>
  );
};

export default Page;
