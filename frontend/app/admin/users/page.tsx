import { Suspense } from "react";
import { UsersContent, UsersContentSkeleton } from "@/components/users";
import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Админ | Пользователи",
};

export const dynamic = "force-dynamic";

type PageProps = {
  searchParams: Promise<{
    page?: string;
    search?: string;
    role?: string;
  }>;
};

export default async function AdminUsersPage({ searchParams }: PageProps) {
  const resolvedSearchParams = await searchParams;
  const page = Number(resolvedSearchParams.page) || 1;
  const search = resolvedSearchParams.search || undefined;
  const role = resolvedSearchParams.role || undefined;

  return (
    <Suspense fallback={<UsersContentSkeleton />}>
      <UsersContent page={page} search={search} role={role} />
    </Suspense>
  );
}
