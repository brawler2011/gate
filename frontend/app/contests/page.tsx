import { DefaultLayout } from '@/components/shared';
import { ContestsContentSkeleton } from '@/components/contests/ContestsContentSkeleton';
import { PublicContestsWrapper } from '@/components/contests/PublicContestsWrapper';
import { UserContestsWrapper } from '@/components/contests/UserContestsWrapper';
import { ContestsHeader } from '@/components/contests/ContestsHeader';
import { ContestsTabs } from '@/components/contests/ContestsTabs';
import { ContestsPageWrapper } from '@/components/contests/ContestsPageWrapper';
import { ErrorDisplay } from '@/components/shared/ErrorDisplay';
import { getPublicContests, getUserContests } from "@/lib/actions";
import { getCurrentUser } from "@/lib/auth";
import { Container, Stack } from "@mantine/core";
import { Metadata } from "next";
import { Suspense } from "react";

export const metadata: Metadata = {
  title: "Контесты",
  description: "Список доступных контестов по программированию.",
};

type Props = {
  searchParams: Promise<{ page?: string; view?: string; search?: string }>;
};

const PublicContestsView = async ({
  page,
  search,
}: {
  page: number;
  search?: string;
}) => {
  const [error, contestsData] = await getPublicContests(page, 10, search);
  if (error) return <ErrorDisplay error={error} />;

  return (
    <PublicContestsWrapper
      contests={contestsData!.contests}
      pagination={contestsData!.pagination}
    />
  );
};

const UserContestsView = async ({
  page,
  search,
  userId,
}: {
  page: number;
  search?: string;
  userId: string;
}) => {
  const [error, contestsData] = await getUserContests(userId, page, 10, search);
  if (error) return <ErrorDisplay error={error} />;

  return (
    <UserContestsWrapper
      contests={contestsData!.contests}
      pagination={contestsData!.pagination}
    />
  );
};

const ContestsPageContent = async ({
  page,
  view,
  search,
}: {
  page: number;
  view: string;
  search?: string;
}) => {
  const currentUser = await getCurrentUser();
  const authenticated = currentUser !== null;

  return (
    <ContestsPageWrapper>
      <Stack gap="md">
        <ContestsHeader />
        <ContestsTabs isAuthenticated={authenticated} />
        {view === "user" && currentUser ? (
          <Suspense fallback={<ContestsContentSkeleton />}>
            <UserContestsView
              page={page}
              search={search}
              userId={currentUser.id}
            />
          </Suspense>
        ) : (
          <Suspense fallback={<ContestsContentSkeleton />}>
            <PublicContestsView page={page} search={search} />
          </Suspense>
        )}
      </Stack>
    </ContestsPageWrapper>
  );
};

const Page = async (props: Props) => {
  const resolvedSearchParams = await props.searchParams;
  const page = Number(resolvedSearchParams.page) || 1;
  const view = resolvedSearchParams.view || "public";
  const search = resolvedSearchParams.search;

  return (
    <DefaultLayout>
      <Container size="lg" py="lg">
        <ContestsPageContent page={page} view={view} search={search} />
      </Container>
    </DefaultLayout>
  );
};

export default Page;
