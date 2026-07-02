"use client";

import { DefaultLayout } from '@/components/shared';
import { UsersContent, UsersContentSkeleton } from '@/components/users';
import { AdminBlogsContent, AdminContestsContent, AdminTabs } from '@/components/admin';
import { Container, Skeleton, Stack } from "@mantine/core";
import { Suspense, useEffect } from "react";
import { useSearchParams } from "next/navigation";

export function AdminContestsContentSkeleton() {
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

export function AdminBlogsContentSkeleton() {
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

export default function AdminPageContent() {
  const searchParams = useSearchParams();
  const page = Number(searchParams.get("page")) || 1;
  const search = searchParams.get("search") || undefined;
  const role = searchParams.get("role") || undefined;
  const view = searchParams.get("view") || "users";

  useEffect(() => {
    document.title = "Админ";
  }, []);

  return (
    <DefaultLayout>
      <Container size="xl" pt="lg">
        <AdminTabs />
      </Container>
      {view === "blogs" ? (
        <Suspense fallback={<AdminBlogsContentSkeleton />}>
          <AdminBlogsContent page={page} search={search} />
        </Suspense>
      ) : view === "contests" ? (
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
