import {Container, Skeleton, Stack} from "@mantine/core";
import {Suspense} from "react";
import {useSearchParams, Navigate} from "react-router-dom";

import {AdminBlogsContent} from "@/components/admin";
import {parsePage} from "@/lib/lib";

import type {ReactNode} from "react";

const AdminBlogsContentSkeleton = () => {
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

const AdminBlogsPage = (): ReactNode => {
  const [urlSearch] = useSearchParams();
  const searchParams = Object.fromEntries(urlSearch);
  const resolvedSearchParams = searchParams;
  const page = parsePage(resolvedSearchParams.page);
  if (!page) {
    return <Navigate to="/admin/blogs" replace />;
  }
  const search = resolvedSearchParams.search || undefined;

  return (
    <Suspense fallback={<AdminBlogsContentSkeleton />}>
      <title>Блог</title>
      <AdminBlogsContent page={page} search={search} />
    </Suspense>
  );
};

export default AdminBlogsPage;
