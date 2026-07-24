import { DefaultLayout } from "@/components/shared";

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return (
    <DefaultLayout>
      {children}
    </DefaultLayout>
  );
}
