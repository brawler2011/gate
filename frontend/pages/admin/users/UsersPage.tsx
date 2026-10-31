import {Suspense} from "react";
import {useSearchParams, Navigate} from "react-router-dom";

import {UsersContent, UsersContentSkeleton} from "@/components/users";
import {parsePage} from "@/lib/lib";

import type {ReactNode} from "react";

const AdminUsersPage = (): ReactNode => {
  const [urlSearch] = useSearchParams();
  const searchParams = Object.fromEntries(urlSearch);
  const resolvedSearchParams = searchParams;
  const page = parsePage(resolvedSearchParams.page);
  if (!page) {
    return <Navigate to="/admin/users" replace />;
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
