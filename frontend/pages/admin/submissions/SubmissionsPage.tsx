import {Container, Skeleton, Stack} from "@mantine/core";
import {Suspense} from "react";
import {useSearchParams, Navigate} from "react-router-dom";

import {AdminSubmissionsContent} from "@/components/admin";
import {parsePage} from "@/lib/lib";

import type {ReactNode} from "react";

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

const AdminSubmissionsPage = (): ReactNode => {
  const [urlSearch] = useSearchParams();
  const searchParams = Object.fromEntries(urlSearch);
  const resolvedSearchParams = searchParams;
  const page = parsePage(resolvedSearchParams.page);
  if (!page) {
    return <Navigate to="/admin/submissions" replace />;
  }

  return (
    <Suspense fallback={<AdminSubmissionsContentSkeleton />}>
      <title>Посылки</title>
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
