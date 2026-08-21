import {Container} from "@mantine/core";
import {notFound, redirect} from "next/navigation";

import {OrgTeamsTab} from "@/components/orgs/OrgTeamsTab";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api, unwrapAndCache} from "@/lib/api";
import {parsePage} from "@/lib/lib";
import {canManageOrgMembers} from "@/lib/permissions";

import type {Metadata} from "next";
import type {ReactNode} from "react";

export const metadata: Metadata = {
  title: "Команды",
};

type Props = {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ page?: string }>;
};

const getOrganization = unwrapAndCache(api.getOrganization);

const OrgTeamsPage = async ({params, searchParams}: Props): Promise<ReactNode> => {
  const {slug} = await params;
  let decoded = "";
  try {
    decoded = decodeURIComponent(slug);
  } catch {
    notFound();
  }

  if (decoded.startsWith("@")) {
    notFound();
  }

  const {page} = await searchParams;
  const currentPage = parsePage(page);
  if (!currentPage) {
    redirect(`/${decoded}/teams`);
  }

  const orgData = await getOrganization({login: decoded});
  const org = orgData.organization;

  const showMembersTab = await canManageOrgMembers(org.login);
  const [teamsError, teamsData] = await api.listTeams({
    organizationId: org.id,
    page: currentPage,
    pageSize: 20,
  });

  const teams = teamsData?.teams ?? [];

  return (
    <Container size="lg" py="lg">
      {teamsError ? (
        <ErrorDisplay error={teamsError} />
      ) : (
        <OrgTeamsTab
          teams={teams}
          orgLogin={org.login}
          orgId={org.id}
          canManage={showMembersTab}
        />
      )}
    </Container>
  );
};

export default OrgTeamsPage;
