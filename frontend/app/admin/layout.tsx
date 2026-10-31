import {Outlet, Navigate} from "react-router-dom";

import {AdminHeaderNav} from "@/components/admin";
import {DefaultLayout} from "@/components/shared";
import {QueryState} from "@/components/shared/QueryState";
import {useSession} from "@/contexts/SessionContext";

import type {ReactNode} from "react";

const AdminLayout = (): ReactNode => {
  const {user, isLoading} = useSession();
  if (isLoading) {
    return <QueryState />;
  }
  if (user?.role !== "admin") {
    return <Navigate to="/" replace />;
  }
  return (
    <DefaultLayout headerUser={user} headerSecondaryNav={<AdminHeaderNav />}>
      <Outlet />
    </DefaultLayout>
  );
};
export default AdminLayout;
