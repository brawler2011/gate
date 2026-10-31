import {Navigate, useLoaderData} from "react-router-dom";

import {DefaultLayout} from "@/components/shared";
import {api} from "@/lib/api";

import {NotificationsClient} from "./NotificationsClient";

import type {ReactNode} from "react";
import type {LoaderFunction} from "react-router-dom";

export const loader = (async () => {
  const {data: me} = await api.GET("/users/me");
  return {me};
}) satisfies LoaderFunction;

const NotificationsPage = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {me} = data;
  const user = me?.user ?? null;
  if (!user) {
    return <Navigate to="/auth/login?return_to=/notifications" replace />;
  }
  return (
    <DefaultLayout headerUser={user}>
      <NotificationsClient />
    </DefaultLayout>
  );
};

export default NotificationsPage;
