import { Suspense } from "react";
import AdminPageContent from "./AdminPageContent";
import { DefaultLayout } from "@/components/shared";
import { Container, Skeleton } from "@mantine/core";
import { UsersContentSkeleton } from "@/components/users";

export const dynamic = 'force-dynamic';

export default function AdminPage() {
  return (
    <DefaultLayout>
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
    </DefaultLayout>
  );
}
