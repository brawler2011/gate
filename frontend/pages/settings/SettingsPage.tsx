import {Navigate, useLoaderData} from "react-router-dom";

import {UserSettingsContent} from "@/components/settings";
import {api} from "@/lib/api";

import type {ReactNode} from "react";
import type {LoaderFunction} from "react-router-dom";

export const loader = (async () => {
  const {data, error} = await api.GET("/users/me");
  return {error, data};
}) satisfies LoaderFunction;

const SettingsPage = (): ReactNode => {
  const loaded = useLoaderData<typeof loader>();
  const {error, data} = loaded;
  if (error || !data?.user || data.user.role === "guest") {
    return <Navigate to="/auth/login?return_to=/settings" replace />;
  }
  return (
    <>
      <title>Настройки</title>
      <UserSettingsContent initialUser={data.user} />
    </>
  );
};

export default SettingsPage;
