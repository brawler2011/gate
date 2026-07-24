import { Suspense } from "react";
import { AdminOrgsContent } from "@/components/admin";
import { Container, Skeleton, Stack } from "@mantine/core";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Админ | Организации",
};

export const dynamic = "force-dynamic";

function AdminOrgsContentSkeleton() {
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
}

type PageProps = {
  searchParams: Promise<{
    page?: string;
    search?: string;
  }>;
};

export default async function AdminOrgsPage({ searchParams }: PageProps) {
  const resolvedSearchParams = await searchParams;
  const page = Number(resolvedSearchParams.page) || 1;
  const search = resolvedSearchParams.search || undefined;

  return (
    <Suspense fallback={<AdminOrgsContentSkeleton />}>
      <AdminOrgsContent page={page} search={search} />
    </Suspense>
  );
}
