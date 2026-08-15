import {DefaultLayout} from "@/components/shared";

import type {ReactNode} from "react";

const AdminLayout = ({children}: { children: React.ReactNode }): ReactNode => {
  return (
    <DefaultLayout>
      {children}
    </DefaultLayout>
  );
};

export default AdminLayout;
