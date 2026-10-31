import {UserSettingsContent} from "@/components/settings";
import {api} from "@/lib/api";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {redirect} from "@/lib/routeErrors";

import type {ReactNode} from "react";

const loadSettingsPage = async () => {
  const [error, data] = await api.getMe();
  return {error, data};
};

const SettingsPage = (): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/settings/page.tsx"], () => loadSettingsPage());
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {error, data} = routeQuery.data;
  if (error || !data?.user || data.user.role === "guest") {
    redirect("/auth/login?return_to=/settings");
  }
  return <UserSettingsContent initialUser={data.user} />;
};

export default SettingsPage;
