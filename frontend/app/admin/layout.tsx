import {DefaultLayout} from "@/components/shared";

const AdminLayout = ({children}: { children: React.ReactNode }) => {
  return (
    <DefaultLayout>
      {children}
    </DefaultLayout>
  );
};

export default AdminLayout;
