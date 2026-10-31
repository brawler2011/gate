import {Container, Skeleton, Stack} from "@mantine/core";
import {Suspense} from "react";

import {AdminBlogsContent} from "@/components/admin";
import {parsePage} from "@/lib/lib";
import {redirect} from "@/lib/routeErrors";

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

type PageProps = {
  searchParams: {
    page?: string;
    search?: string;
  };
};

const AdminBlogsPage = ({searchParams}: PageProps): ReactNode => {
  const resolvedSearchParams = searchParams;
  const page = parsePage(resolvedSearchParams.page);
  if (!page) {
    redirect("/admin/blogs");
  }
  const search = resolvedSearchParams.search || undefined;

  return (
    <Suspense fallback={<AdminBlogsContentSkeleton />}>
      <AdminBlogsContent page={page} search={search} />
    </Suspense>
  );
};

export default AdminBlogsPage;
