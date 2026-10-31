import {AdminDashboardContent} from "@/components/admin";

import type {ReactNode} from "react";

const AdminPage = (): ReactNode => {
  return (
    <>
      <title>Администрирование</title>
      <AdminDashboardContent />
    </>
  );
};

export default AdminPage;
