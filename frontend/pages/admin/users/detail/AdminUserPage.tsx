import {Container} from "@mantine/core";
import {useLoaderData, redirect} from "react-router-dom";

import {AdminUserEditContent} from "@/components/admin";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";
import NotFound from "@/pages/NotFoundPage";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams}: LoaderFunctionArgs) => {
  const {username} = routeParams as {username: string};
  const decoded = username;

  // Check admin rights
  const {data: meData, error: meError} = await api.GET("/users/me");
  if (meError || !meData?.user || meData.user.role !== "admin") {
    throw redirect("/auth/login?return_to=/admin/users/" + encodeURIComponent(decoded));
  }

  const {data, error, response} = await api.GET("/users/{username}", {
    params: {path: {username: decoded}},
  });
  const err = error
    ? {
      status: response.status,
      message: (error as {message?: string})?.message || response.statusText,
    }
    : null;
  return {error: err, data};
}) satisfies LoaderFunction;

const AdminUserEditPage = (): ReactNode => {
  const loaded = useLoaderData<typeof loader>();
  const {error, data} = loaded;
  if (error) {
    if (error.status === 404) {
      return <NotFound />;
    }
    return (
      <Container size="sm" py="lg">
        <ErrorDisplay error={error} />
      </Container>
    );
  }
  return (
    <>
      <title>{data!.user.username}</title>
      <AdminUserEditContent user={data!.user} />
    </>
  );
};

export default AdminUserEditPage;
