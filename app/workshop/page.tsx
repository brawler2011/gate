import { DefaultLayout } from "@/components/Layout";
import { WorkshopContestsContentSkeleton } from "@/components/WorkshopPage/WorkshopContestsContentSkeleton";
import { WorkshopContestsWrapper } from "@/components/WorkshopPage/WorkshopContestsWrapper";
import { WorkshopHeader } from "@/components/WorkshopPage/WorkshopHeader";
import { WorkshopPageWrapper } from "@/components/WorkshopPage/WorkshopPageWrapper";
import { WorkshopProblemsContentSkeleton } from "@/components/WorkshopPage/WorkshopProblemsContentSkeleton";
import { WorkshopProblemsWrapper } from "@/components/WorkshopPage/WorkshopProblemsWrapper";
import { WorkshopTabs } from "@/components/WorkshopPage/WorkshopTabs";
import { ErrorDisplay } from "@/components/ErrorDisplay";
import { getContests, getProblems } from "@/lib/actions";
import { isAuthenticated } from "@/lib/auth";
import { Container, Stack } from "@mantine/core";
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
  authenticated,
}: {
  page: number;
  authenticated: boolean;
}) => {
  const [error, problemsData] = await getProblems(page, 20, undefined, undefined, true);
  if (error) return <ErrorDisplay error={error} />;

  return (
    <WorkshopProblemsWrapper
      problems={problemsData!.problems}
      pagination={problemsData!.pagination}
      isAuthenticated={authenticated}
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
  const [error, contestsData] = await getContests(page, 10, search);
  if (error) return <ErrorDisplay error={error} />;

  return (
    <WorkshopContestsWrapper
      contests={contestsData!.contests}
      pagination={contestsData!.pagination}
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
  const authenticated = await isAuthenticated();

  return (
    <WorkshopPageWrapper>
      <Stack gap="md">
        <WorkshopHeader isAuthenticated={authenticated} />
        <WorkshopTabs isAuthenticated={authenticated} />
        {view === "problems" ? (
          <Suspense fallback={<WorkshopProblemsContentSkeleton />}>
            <ProblemsView
              page={page}
              authenticated={authenticated}
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
