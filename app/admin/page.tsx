import { DefaultLayout } from "@/components/Layout";
import { UsersContent } from "@/components/UsersContent";
import { UsersContentSkeleton } from "@/components/UsersPage";
import { AdminContestsContent, AdminTabs } from "@/components/AdminPage";
import { Container, Skeleton, Stack } from "@mantine/core";
import { Metadata } from "next";
import { Suspense } from "react";

export const metadata: Metadata = {
  title: "Админ",
  description: "Административная панель",
};

type SearchParams = Promise<{
  page?: string;
  search?: string;
  role?: string;
  view?: string;
}>;

type Props = {
  searchParams: SearchParams;
};

function AdminContestsContentSkeleton() {
  return (
    <Container size="xl" py="md">
      <Stack gap="md">
        <Skeleton height={30} width={150} radius="sm" />
        <Skeleton height={36} width={400} radius="sm" />
        <Stack gap="sm">
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
        </Stack>
      </Stack>
    </Container>
  );
}

export default async function AdminPage({ searchParams }: Props) {
  const params = await searchParams;
  const page = Number(params.page) || 1;
  const search = params.search;
  const role = params.role;
  const view = params.view || "users";

  return (
    <DefaultLayout>
      <Container size="xl" pt="lg">
        <AdminTabs />
      </Container>
      {view === "contests" ? (
        <Suspense fallback={<AdminContestsContentSkeleton />}>
          <AdminContestsContent page={page} search={search} />
        </Suspense>
      ) : (
        <Suspense fallback={<UsersContentSkeleton />}>
          <UsersContent page={page} search={search} role={role} />
        </Suspense>
      )}
    </DefaultLayout>
  );
}
