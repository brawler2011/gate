import {redirect} from "next/navigation";

import {DefaultLayout} from "@/components/shared";
import {api} from "@/lib/api";

import {NotificationsClient} from "./NotificationsClient";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Уведомления",
};

const NotificationsPage = async (): Promise<ReactNode> => {
  const [, me] = await api.getMe();
  const user = me?.user ?? null;

  if (!user) {
    redirect("/auth/login?return_to=/notifications");
  }

  return (
    <DefaultLayout headerUser={user}>
      <NotificationsClient />
    </DefaultLayout>
  );
};

export default NotificationsPage;
