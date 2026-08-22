import {Container} from "@mantine/core";
import {notFound, redirect} from "next/navigation";

import {AdminUserEditContent} from "@/components/admin";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ username: string }>;
};

export const dynamic = "force-dynamic";

export const generateMetadata = async ({params}: Props): Promise<Metadata> => {
  const {username} = await params;
  return {
    title: `Пользователь @${decodeURIComponent(username)} | Администрирование`,
  };
};

const AdminUserEditPage = async ({params}: Props): Promise<ReactNode> => {
  const {username} = await params;
  let decoded = "";
  try {
    decoded = decodeURIComponent(username);
  } catch {
    notFound();
  }

  // Check admin rights
  const [meError, meData] = await api.getMe();
  if (meError || !meData?.user || meData.user.role !== "admin") {
    redirect("/auth/login?return_to=/admin/users/" + encodeURIComponent(decoded));
  }

  const [error, data] = await api.getUser({username: decoded});
  if (error) {
    if (error.status === 404) {
      notFound();
    }
    return (
      <Container size="sm" py="lg">
        <ErrorDisplay error={error} />
      </Container>
    );
  }

  return <AdminUserEditContent user={data!.user} />;
};

export default AdminUserEditPage;
