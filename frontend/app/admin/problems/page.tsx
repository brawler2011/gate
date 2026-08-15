import {Container, Skeleton, Stack} from "@mantine/core";
import {redirect} from "next/navigation";
import {Suspense} from "react";

import {AdminProblemsContent} from "@/components/admin";
import {parsePage} from "@/lib/lib2";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Админ | Задачи",
};

export const dynamic: string = "force-dynamic";

const AdminProblemsContentSkeleton = () => {
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
    search?: string;
  }>;
};

const AdminProblemsPage = async ({searchParams}: PageProps): Promise<ReactNode> => {
  const resolvedSearchParams = await searchParams;
  const page = parsePage(resolvedSearchParams.page);
  if (!page) {
    redirect("/admin/problems");
  }
  const search = resolvedSearchParams.search || undefined;

  return (
    <Suspense fallback={<AdminProblemsContentSkeleton />}>
      <AdminProblemsContent page={page} search={search} />
    </Suspense>
  );
};

export default AdminProblemsPage;
