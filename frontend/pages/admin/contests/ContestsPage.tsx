import {Container, Skeleton, Stack} from "@mantine/core";
import {Suspense} from "react";
import {useSearchParams, Navigate} from "react-router-dom";

import {AdminContestsContent} from "@/components/admin";
import {parsePage} from "@/lib/lib";

import type {ReactNode} from "react";

const AdminContestsContentSkeleton = () => {
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

const AdminContestsPage = (): ReactNode => {
  const [urlSearch] = useSearchParams();
  const searchParams = Object.fromEntries(urlSearch);
  const resolvedSearchParams = searchParams;
  const page = parsePage(resolvedSearchParams.page);
  if (!page) {
    return <Navigate to="/admin/contests" replace />;
  }
  const search = resolvedSearchParams.search || undefined;

  return (
    <Suspense fallback={<AdminContestsContentSkeleton />}>
      <title>Контесты</title>
      <AdminContestsContent page={page} search={search} />
    </Suspense>
  );
};

export default AdminContestsPage;
