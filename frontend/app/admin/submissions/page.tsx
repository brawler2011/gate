import {Container, Skeleton, Stack} from "@mantine/core";
import {redirect} from "next/navigation";
import {Suspense} from "react";

import {AdminSubmissionsContent} from "@/components/admin";
import {parsePage} from "@/lib/lib";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Админ | Посылки",
};

export const dynamic: string = "force-dynamic";

const AdminSubmissionsContentSkeleton = () => {
  return (
    <Container size="xl" py="md">
      <Stack gap="md">
        <Skeleton height={36} width={400} radius="sm" />
        <Stack gap="sm">
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
        </Stack>
      </Stack>
    </Container>
  );
};

type PageProps = {
  searchParams: Promise<{
    page?: string;
    state?: string;
    language?: string;
    contestId?: string;
    problemId?: string;
    userId?: string;
  }>;
};

const AdminSubmissionsPage = async ({searchParams}: PageProps): Promise<ReactNode> => {
  const resolvedSearchParams = await searchParams;
  const page = parsePage(resolvedSearchParams.page);
  if (!page) {
    redirect("/admin/submissions");
  }

  return (
    <Suspense fallback={<AdminSubmissionsContentSkeleton />}>
      <AdminSubmissionsContent
        page={page}
        state={resolvedSearchParams.state}
        language={resolvedSearchParams.language}
        contestId={resolvedSearchParams.contestId}
        problemId={resolvedSearchParams.problemId}
        userId={resolvedSearchParams.userId}
      />
    </Suspense>
  );
};

export default AdminSubmissionsPage;
