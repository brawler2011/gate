import { Container, Skeleton, Stack } from "@mantine/core";
import { redirect } from "next/navigation";
import { Suspense } from "react";

import { AdminSubmissionsContent } from "@/components/admin";
import { parsePage } from "@/lib/lib2";

import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Админ | Посылки",
};

export const dynamic = "force-dynamic";

const AdminSubmissionsContentSkeleton = () => {
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
        </Stack>
      </Stack>
    </Container>
  );
};

type PageProps = {
  searchParams: Promise<{
    page?: string;
  }>;
};

const AdminSubmissionsPage = async ({ searchParams }: PageProps) => {
  const resolvedSearchParams = await searchParams;
  const page = parsePage(resolvedSearchParams.page);
  if (!page) {
    redirect("/admin/submissions");
  }

  return (
    <Suspense fallback={<AdminSubmissionsContentSkeleton />}>
      <AdminSubmissionsContent page={page} />
    </Suspense>
  );
};

export default AdminSubmissionsPage;
