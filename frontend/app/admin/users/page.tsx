import {Suspense} from "react";

import {UsersContent, UsersContentSkeleton} from "@/components/users";
import {parsePage} from "@/lib/lib";
import {redirect} from "@/lib/routeErrors";

import type {ReactNode} from "react";

type PageProps = {
  searchParams: {
    page?: string;
    search?: string;
    role?: string;
  };
};

const AdminUsersPage = ({searchParams}: PageProps): ReactNode => {
  const resolvedSearchParams = searchParams;
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
