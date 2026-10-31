import {Container} from "@mantine/core";

import {OrgTeamsTab} from "@/components/orgs/OrgTeamsTab";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api, unwrap} from "@/lib/api";
import {parsePage} from "@/lib/lib";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {canManageOrgMembers} from "@/lib/permissions";
import {notFound, redirect} from "@/lib/routeErrors";

import type {ReactNode} from "react";

const loadOrgTeamsPage = async ({params, searchParams}: Props) => {
  const {slug} = params;
  let decoded = "";
  try {
    decoded = decodeURIComponent(slug);
  } catch {
    notFound();
  }

  if (decoded.startsWith("@")) {
    notFound();
  }

  const {page} = searchParams;
  const currentPage = parsePage(page);
  if (!currentPage) {
    redirect(`/${decoded}/teams`);
  }

  const orgData = await getOrganization(decoded);
  const org = orgData.organization;

  const showMembersTab = await canManageOrgMembers(org.login);
  const [teamsError, teamsData] = await api.listTeams({
    page: currentPage,
    pageSize: 20, organization_id: org.id
  });
  return {
    slug,
    decoded,
    page,
    currentPage,
    orgData,
    org,
    showMembersTab,
    teamsError,
    teamsData,
  };
};

type Props = {
  params: { slug: string };
  searchParams: { page?: string };
};

const getOrganization = unwrap(api.getOrganization);

const OrgTeamsPage = ({params, searchParams}: Props): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/[slug]/(org)/teams/page.tsx", params, searchParams], () =>
    loadOrgTeamsPage({params, searchParams}),
  );
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {org, showMembersTab, teamsError, teamsData} = routeQuery.data;
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
