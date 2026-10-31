import {DefaultLayout} from "@/components/shared";
import {api} from "@/lib/api";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {redirect} from "@/lib/routeErrors";

import {NotificationsClient} from "./NotificationsClient";

import type {ReactNode} from "react";

const loadNotificationsPage = async () => {
  const [, me] = await api.getMe();
  return {me};
};

const NotificationsPage = (): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/notifications/page.tsx"], () => loadNotificationsPage());
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {me} = routeQuery.data;
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
