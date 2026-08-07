import { Container } from "@mantine/core";
import { notFound } from "next/navigation";

import { OrgTeamsTab } from "@/components/orgs/OrgTeamsTab";
import { DefaultLayout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { api } from "@/lib/api";
import { buildOrgHeaderNav } from "@/lib/org-header-nav";
import { canManageOrgMembers } from "@/lib/org-permissions";

type Props = {
  params: Promise<{ org_id: string }>;
  searchParams: Promise<{ page?: string }>;
};

const OrgTeamsPage = async ({ params, searchParams }: Props) => {
  const { org_id } = await params;
  const { page } = await searchParams;
  const showMembersTab = await canManageOrgMembers(org_id);
  const orgHeaderNav = buildOrgHeaderNav({
    orgId: org_id,
    activeTab: "teams",
    showMembersTab,
  });
  const currentPage = Number(page) > 0 ? Number(page) : 1;

  const [orgError, orgData] = await api.getOrganization({ id: org_id });
  if (orgError) {
    if (orgError.status === 404) {
      notFound();
    }
    return (
      <DefaultLayout headerOrganizationId={org_id}>
        <Container size="lg" py="lg">
          <ErrorDisplay error={orgError} />
        </Container>
      </DefaultLayout>
    );
  }

  const [teamsError, teamsData] = await api.listTeams({ organizationId: org_id, page: currentPage, pageSize: 20 });

  const org = orgData!.organization;
  const teams = teamsData?.teams ?? [];

  return (
    <DefaultLayout
      headerSecondaryNavItems={orgHeaderNav}
      headerOrganization={{ id: org.id, name: org.name }}
    >
      <Container size="lg" py="lg">
        {teamsError ? (
          <ErrorDisplay error={teamsError} />
        ) : (
          <OrgTeamsTab teams={teams} orgId={org_id} />
        )}
      </Container>
    </DefaultLayout >
  );
};

export default OrgTeamsPage;
