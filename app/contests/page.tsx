import { DefaultLayout } from "@/components/Layout";
import { ContestsContentSkeleton } from "@/components/ContestsPage/ContestsContentSkeleton";
import { PublicContestsWrapper } from "@/components/ContestsPage/PublicContestsWrapper";
import { UserContestsWrapper } from "@/components/ContestsPage/UserContestsWrapper";
import { ContestsHeader } from "@/components/ContestsPage/ContestsHeader";
import { ContestsTabs } from "@/components/ContestsPage/ContestsTabs";
import { ContestsPageWrapper } from "@/components/ContestsPage/ContestsPageWrapper";
import { getPublicContests, getUserContests } from "@/lib/actions";
import { getCurrentUser } from "@/lib/auth";
import { Alert, Center, Container, Stack } from "@mantine/core";
import { IconAlertCircle } from "@tabler/icons-react";
import { Metadata } from "next";
import { Suspense } from "react";

export const metadata: Metadata = {
  title: "Контесты",
  description: "",
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
  const contestsData = await getPublicContests(page, 10, search);

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
    <PublicContestsWrapper
      contests={contestsData.contests}
      pagination={contestsData.pagination}
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
  const contestsData = await getUserContests(userId, page, 10, search);

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
    <UserContestsWrapper
      contests={contestsData.contests}
      pagination={contestsData.pagination}
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
  const isAuthenticated = !!currentUser;

  return (
    <ContestsPageWrapper>
      <Stack gap="md">
        <ContestsHeader />
        <ContestsTabs isAuthenticated={isAuthenticated} />
        {view === "user" && isAuthenticated ? (
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

