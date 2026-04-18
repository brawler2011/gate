import { OrgTabs } from "@/components/orgs";
import { DefaultLayout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import {
  getContests,
  getOrganization,
  getProblems,
  listOrganizationMembers,
  listTeams,
} from "@/lib/actions";
import {
  buildOrgHeaderNav,
  ORG_OVERVIEW_TABS,
  type OrgOverviewTab,
} from "@/lib/org-header-nav";
import { Container, Stack, Text, Title } from "@mantine/core";
import { notFound } from "next/navigation";

function isOrgOverviewTab(value: string | undefined): value is OrgOverviewTab {
  return ORG_OVERVIEW_TABS.includes(value as OrgOverviewTab);
}

type Props = {
  params: Promise<{ org_id: string }>;
  searchParams: Promise<{ tab?: string }>;
};

export default async function OrgPage({ params, searchParams }: Props) {
  const { org_id } = await params;
  const { tab } = await searchParams;
  const activeTab: OrgOverviewTab = isOrgOverviewTab(tab) ? tab : "contests";
  const orgHeaderNav = buildOrgHeaderNav({ orgId: org_id, activeTab });

  const [orgError, orgData] = await getOrganization(org_id);
  if (orgError) {
    if (orgError.status === 404) notFound();
    return (
      <DefaultLayout>
        <Container size="lg" py="lg">
          <ErrorDisplay error={orgError} />
        </Container>
      </DefaultLayout>
    );
  }

  const [
    [membersError, membersData],
    [teamsError, teamsData],
    [problemsError, problemsData],
    [contestsError, contestsData],
  ] = await Promise.all([
    listOrganizationMembers(org_id, 1, 20),
    listTeams(org_id, 1, 20),
    getProblems(1, 20, undefined, undefined, undefined, org_id),
    getContests(1, 20, undefined, org_id),
  ]);

  const org = orgData!.organization;
  const members = membersData?.members ?? [];
  const teams = teamsData?.teams ?? [];
  const problems = problemsData?.problems ?? [];
  const contests = contestsData?.contests ?? [];

  return (
    <DefaultLayout headerSecondaryNavItems={orgHeaderNav}>
      <Container size="lg" py="lg">
        <Stack gap="md">
          <div>
            <Title order={2}>{org.name}</Title>
            {org.description && (
              <Text c="dimmed" size="sm">
                {org.description}
              </Text>
            )}
          </div>

          <OrgTabs
            members={members}
            teams={teams}
            problems={problems}
            contests={contests}
            orgId={org_id}
            activeTab={activeTab}
            membersError={membersError}
            teamsError={teamsError}
            problemsError={problemsError}
            contestsError={contestsError}
          />
        </Stack>
      </Container>
    </DefaultLayout>
  );
}
