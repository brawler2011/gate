import {Container} from "@mantine/core";
import {useLoaderData} from "react-router-dom";

import {OrgMembersManagement} from "@/components/orgs/OrgMembersManagement";
import {canOpenOrgMembers, getOrgPermissions} from "@/lib/permissions";
import NotFound from "@/pages/NotFoundPage";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams}: LoaderFunctionArgs) => {
  const {slug} = routeParams as {slug: string};
  const decoded = slug;

  if (decoded.startsWith("@")) {
    throw new Response("Страница не найдена", {status: 404});
  }

  const permissions = await getOrgPermissions(decoded);
  return {decoded, permissions};
}) satisfies LoaderFunction;

const OrgMembersPage = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {decoded, permissions} = data;
  if (!permissions || !canOpenOrgMembers(permissions)) {
    return <NotFound />;
  }
  return (
    <Container size="lg" py="lg">
      <title>Участники</title>
      <OrgMembersManagement orgLogin={decoded} canManageMembers={permissions.manage_members} />
    </Container>
  );
};

export default OrgMembersPage;
