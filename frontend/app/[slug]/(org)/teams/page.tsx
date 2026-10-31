import {Container} from "@mantine/core";
import {useLoaderData, redirect} from "react-router-dom";

import {OrgTeamsTab} from "@/components/orgs/OrgTeamsTab";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api, unwrap} from "@/lib/api";
import {parsePage} from "@/lib/lib";
import {canManageOrgMembers} from "@/lib/permissions";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams, request}: LoaderFunctionArgs) => {
  const searchParams = Object.fromEntries(new URL(request.url).searchParams);
  const {slug} = routeParams as {slug: string};
  const decoded = slug;

  if (decoded.startsWith("@")) {
    throw new Response("Страница не найдена", {status: 404});
  }

  const {page} = searchParams;
  const currentPage = parsePage(page);
  if (!currentPage) {
    throw redirect(`/${decoded}/teams`);
  }

  const orgData = await getOrganization(decoded);
  const org = orgData.organization;

  const showMembersTab = await canManageOrgMembers(org.login);
  const [teamsError, teamsData] = await api.listTeams({
    page: currentPage,
    pageSize: 20, organization_id: org.id
  });
  return {org, showMembersTab, teamsError, teamsData};
}) satisfies LoaderFunction;

const getOrganization = unwrap(api.getOrganization);

const OrgTeamsPage = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {org, showMembersTab, teamsError, teamsData} = data;
  const teams = teamsData?.teams ?? [];
  return (
    <Container size="lg" py="lg">
      {teamsError ? (
        <ErrorDisplay error={teamsError} />
      ) : (
        <OrgTeamsTab teams={teams} orgLogin={org.login} orgId={org.id} canManage={showMembersTab} />
      )}
    </Container>
  );
};

export default OrgTeamsPage;
