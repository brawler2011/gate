import {redirect} from "next/navigation";

import {AdminHeaderNav} from "@/components/admin";
import {DefaultLayout} from "@/components/shared";
import {api} from "@/lib/api";

import type {ReactNode} from "react";

const AdminLayout = async ({children}: { children: React.ReactNode }): Promise<ReactNode> => {
  const [, me] = await api.getMe();
  const user = me?.user ?? null;

  if (!user || user.role !== "admin") {
    redirect("/");
  }

  return (
    <DefaultLayout headerSecondaryNav={<AdminHeaderNav />}>
      {children}
    </DefaultLayout>
  );
};

export default AdminLayout;
