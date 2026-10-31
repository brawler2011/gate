import {Container} from "@mantine/core";
import {useLoaderData, redirect} from "react-router-dom";

import NotFound from "@/app/not-found";
import {AdminUserEditContent} from "@/components/admin";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams}: LoaderFunctionArgs) => {
  const {username} = routeParams as {username: string};
  const decoded = username;

  // Check admin rights
  const [meError, meData] = await api.getMe();
  if (meError || !meData?.user || meData.user.role !== "admin") {
    throw redirect("/auth/login?return_to=/admin/users/" + encodeURIComponent(decoded));
  }

  const [error, data] = await api.getUser(decoded);
  return {error, data};
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
  return <AdminUserEditContent user={data!.user} />;
};

export default AdminUserEditPage;
