import {Container} from "@mantine/core";
import {redirect} from "next/navigation";

import {OrgTeamsTab} from "@/components/orgs/OrgTeamsTab";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";
import {parsePage} from "@/lib/lib";
import {canManageOrgMembers} from "@/lib/permissions";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Команды",
};

type Props = {
  params: Promise<{ org_id: string }>;
  searchParams: Promise<{ page?: string }>;
};

const OrgTeamsPage = async ({params, searchParams}: Props): Promise<ReactNode> => {
  const {org_id} = await params;
  const {page} = await searchParams;
  const currentPage = parsePage(page);
  if (!currentPage) {
    redirect(`/orgs/${org_id}/teams`);
  }

  const showMembersTab = await canManageOrgMembers(org_id);
  const [teamsError, teamsData] = await api.listTeams({
    organizationId: org_id,
    page: currentPage,
    pageSize: 20,
  });

  const teams = teamsData?.teams ?? [];

  return (
    <Container size="lg" py="lg">
      {teamsError ? (
        <ErrorDisplay error={teamsError} />
      ) : (
        <OrgTeamsTab teams={teams} orgId={org_id} canManage={showMembersTab} />
      )}
    </Container>
  );
};

export default OrgTeamsPage;
