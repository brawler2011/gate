import {Container} from "@mantine/core";
import {useLoaderData} from "react-router-dom";

import {OrgMembersManagement} from "@/components/orgs/OrgMembersManagement";
import {canManageOrgMembers} from "@/lib/permissions";
import NotFound from "@/pages/NotFoundPage";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams}: LoaderFunctionArgs) => {
  const {slug} = routeParams as {slug: string};
  const decoded = slug;

  if (decoded.startsWith("@")) {
    throw new Response("Страница не найдена", {status: 404});
  }

  const canManage = await canManageOrgMembers(decoded);
  return {decoded, canManage};
}) satisfies LoaderFunction;

const OrgMembersPage = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {decoded, canManage} = data;
  if (!canManage) {
    return <NotFound />;
  }
  return (
    <Container size="lg" py="lg">
      <OrgMembersManagement orgLogin={decoded} />
    </Container>
  );
};

export default OrgMembersPage;
