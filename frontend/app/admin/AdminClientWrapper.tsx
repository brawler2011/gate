"use client";

import dynamic from "next/dynamic";
import { DefaultLayout } from "@/components/shared";
import { Container, Skeleton } from "@mantine/core";
import { UsersContentSkeleton } from "@/components/users";

import { Suspense } from "react";

const AdminPageContent = dynamic(() => import("./AdminPageContent"), {
  ssr: false,
  loading: () => (
    <DefaultLayout>
      <Container size="xl" pt="lg">
        <Skeleton height={40} radius="sm" />
      </Container>
      <UsersContentSkeleton />
    </DefaultLayout>
  ),
});

export default function AdminClientWrapper() {
  return (
    <Suspense fallback={
      <DefaultLayout>
        <Container size="xl" pt="lg">
          <Skeleton height={40} radius="sm" />
        </Container>
        <UsersContentSkeleton />
      </DefaultLayout>
    }>
      <AdminPageContent />
    </Suspense>
  );
}
