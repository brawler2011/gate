import {AdminDashboardContent} from "@/components/admin";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Админ | Обзор",
};

export const dynamic: string = "force-dynamic";

const AdminPage = (): ReactNode => {
  return <AdminDashboardContent />;
};

export default AdminPage;
