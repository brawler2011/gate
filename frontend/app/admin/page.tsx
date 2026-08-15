import {redirect} from "next/navigation";

import type {ReactNode} from "react";

const AdminPage = (): ReactNode => {
  redirect("/admin/users");
};

export default AdminPage;
