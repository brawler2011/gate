import { DefaultLayout } from '@/components/shared';
import { WorkshopContestsContentSkeleton } from '@/components/workshop/WorkshopContestsContentSkeleton';
import { WorkshopContestsWrapper } from '@/components/workshop/WorkshopContestsWrapper';
import { WorkshopHeader } from '@/components/workshop/WorkshopHeader';
import { WorkshopOrgSelector } from '@/components/workshop/WorkshopOrgSelector';
import { WorkshopPageWrapper } from '@/components/workshop/WorkshopPageWrapper';
import { WorkshopProblemsContentSkeleton } from '@/components/workshop/WorkshopProblemsContentSkeleton';
import { WorkshopProblemsWrapper } from '@/components/workshop/WorkshopProblemsWrapper';
import { WorkshopTabs } from '@/components/workshop/WorkshopTabs';
import { ErrorDisplay } from '@/components/shared/ErrorDisplay';
import { getContests, getProblems, listOrganizations } from "@/lib/actions";
import { isAuthenticated } from "@/lib/auth";
import { Container, Group, Stack } from "@mantine/core";
import { Metadata } from "next";
import { Suspense } from "react";

export const metadata: Metadata = {
  title: "Мастерская",
  description: "",
};

type Props = {
  searchParams: Promise<{ page?: string; view?: string; search?: string; org_id?: string }>;
};

const ProblemsView = async ({
  page,
  authenticated,
  orgId,
}: {
  page: number;
  authenticated: boolean;
  orgId?: string;
}) => {
  const [error, problemsData] = await getProblems(page, 20, undefined, undefined, true, orgId);
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
  orgId,
}: {
  page: number;
  search?: string;
  orgId?: string;
}) => {
  const [error, contestsData] = await getContests(page, 10, search, orgId);
  if (error) return <ErrorDisplay error={error} />;

  return (
    <WorkshopContestsWrapper
      contests={contestsData!.contests}
      pagination={contestsData!.pagination}
      search={search}
    />
  );
};

const WorshopPageContent = async ({
  page,
  view,
  search,
  orgId,
}: {
  page: number;
  view: string;
  search?: string;
  orgId?: string;
}) => {
  const authenticated = await isAuthenticated();
  const [, orgsData] = await listOrganizations(1, 50);
  const orgs = orgsData?.organizations ?? [];

  return (
    <WorkshopPageWrapper>
      <Stack gap="md">
        <WorkshopHeader isAuthenticated={authenticated} orgs={orgs} />
        <Group gap="md">
          <WorkshopTabs isAuthenticated={authenticated} />
          <WorkshopOrgSelector orgs={orgs} selectedOrgId={orgId} />
        </Group>
        {view === "problems" ? (
          <Suspense fallback={<WorkshopProblemsContentSkeleton />}>
            <ProblemsView
              page={page}
              authenticated={authenticated}
              orgId={orgId}
            />
          </Suspense>
        ) : (
          <Suspense fallback={<WorkshopContestsContentSkeleton />}>
            <ContestsView
              page={page}
              search={search}
              orgId={orgId}
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
  const orgId = resolvedSearchParams.org_id;

  return (
    <DefaultLayout>
      <Container size="lg" py="lg">
        <WorshopPageContent page={page} view={view} search={search} orgId={orgId} />
      </Container>
    </DefaultLayout>
  );
};

export default Page;
