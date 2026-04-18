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
import { getCurrentUser } from "@/lib/auth";
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
  searchParams: Promise<{ tab?: string; page?: string; search?: string }>;
};

export default async function OrgPage({ params, searchParams }: Props) {
  const { org_id } = await params;
  const { tab, page, search } = await searchParams;
  const activeTab: OrgOverviewTab = isOrgOverviewTab(tab) ? tab : "contests";
  const orgHeaderNav = buildOrgHeaderNav({ orgId: org_id, activeTab });
  const currentPage = Number(page) > 0 ? Number(page) : 1;
  const contestsSearch = activeTab === "contests" ? search : undefined;
  const contestsPage = activeTab === "contests" ? currentPage : 1;
  const problemsPage = activeTab === "problems" ? currentPage : 1;

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
    currentUser,
  ] = await Promise.all([
    listOrganizationMembers(org_id, 1, 20),
    listTeams(org_id, 1, 20),
    getProblems(problemsPage, 20, undefined, undefined, undefined, org_id),
    getContests(contestsPage, 10, contestsSearch, org_id),
    getCurrentUser(),
  ]);

  const org = orgData!.organization;
  const members = membersData?.members ?? [];
  const teams = teamsData?.teams ?? [];
  const problems = problemsData?.problems ?? [];
  const contests = contestsData?.contests ?? [];
  const problemsPagination = problemsData?.pagination ?? { page: 1, total: 1 };
  const contestsPagination = contestsData?.pagination ?? { page: 1, total: 1 };
  const isAuthenticated = currentUser !== null;

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
            problemsPagination={problemsPagination}
            contests={contests}
            contestsPagination={contestsPagination}
            org={org}
            orgId={org_id}
            isAuthenticated={isAuthenticated}
            search={contestsSearch}
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
