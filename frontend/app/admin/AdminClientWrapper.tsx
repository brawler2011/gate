"use client";

import dynamic from "next/dynamic";
import { Container, Skeleton } from "@mantine/core";
import { UsersContentSkeleton } from "@/components/users";

import { Suspense } from "react";

const AdminPageContent = dynamic(() => import("./AdminPageContent"), {
  ssr: false,
  loading: () => (
    <>
      <Container size="xl" pt="lg">
        <Skeleton height={40} radius="sm" />
      </Container>
      <UsersContentSkeleton />
    </>
  ),
});

export default function AdminClientWrapper() {
  return (
    <Suspense fallback={
      <>
        <Container size="xl" pt="lg">
          <Skeleton height={40} radius="sm" />
        </Container>
        <UsersContentSkeleton />
      </>
    }>
      <AdminPageContent />
    </Suspense>
  );
}
