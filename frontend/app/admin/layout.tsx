import {Outlet} from "react-router-dom";
import {Navigate} from "react-router-dom";

import {AdminHeaderNav} from "@/components/admin";
import {DefaultLayout} from "@/components/shared";
import {useSession} from "@/contexts/SessionContext";
import {PageQueryState} from "@/lib/pageQuery";

import type {ReactNode} from "react";

const AdminLayout = (): ReactNode => {
  const {user, isLoading} = useSession();
  if (isLoading) {
    return <PageQueryState query={{error: null}} />;
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
