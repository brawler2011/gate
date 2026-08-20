import {redirect} from "next/navigation";
import {Suspense} from "react";

import {UsersContent, UsersContentSkeleton} from "@/components/users";
import {parsePage} from "@/lib/lib";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Админ | Пользователи",
};

export const dynamic: string = "force-dynamic";

type PageProps = {
  searchParams: Promise<{
    page?: string;
    search?: string;
    role?: string;
  }>;
};

const AdminUsersPage = async ({searchParams}: PageProps): Promise<ReactNode> => {
  const resolvedSearchParams = await searchParams;
  const page = parsePage(resolvedSearchParams.page);
  if (!page) {
    redirect("/admin/users");
  }
  const search = resolvedSearchParams.search || undefined;
  const role = resolvedSearchParams.role || undefined;

  return (
    <Suspense fallback={<UsersContentSkeleton />}>
      <UsersContent page={page} search={search} role={role} />
    </Suspense>
  );
};

export default AdminUsersPage;
