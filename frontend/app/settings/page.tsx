import {redirect} from "next/navigation";

import {UserSettingsContent} from "@/components/settings";
import {api} from "@/lib/api";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Настройки профиля",
};

export const dynamic = "force-dynamic";

const SettingsPage = async (): Promise<ReactNode> => {
  const [error, data] = await api.getMe();
  if (error || !data?.user || data.user.role === "guest") {
    redirect("/auth/login?return_to=/settings");
  }

  return <UserSettingsContent initialUser={data.user} />;
};

export default SettingsPage;
