import { Container } from "@mantine/core";
import { notFound } from "next/navigation";

import { OrgContestsTab } from "@/components/orgs/OrgContestsTab";
import { DefaultLayout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { getContests, getOrganization } from "@/lib/actions";
import { getCurrentUser } from "@/lib/auth";
import { buildOrgHeaderNav } from "@/lib/org-header-nav";
import { canManageOrgMembers } from "@/lib/org-permissions";

type Props = {
  params: Promise<{ org_id: string }>;
  searchParams: Promise<{ page?: string; search?: string }>;
};

const OrgPage = async ({ params, searchParams }: Props) => {
  const { org_id } = await params;
  const { page, search } = await searchParams;
  const showMembersTab = await canManageOrgMembers(org_id);
  const orgHeaderNav = buildOrgHeaderNav({
    orgId: org_id,
    activeTab: "contests",
    showMembersTab,
  });
  const currentPage = Number(page) > 0 ? Number(page) : 1;

  const [orgError, orgData] = await getOrganization(org_id);
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

  const [
    [contestsError, contestsData],
    currentUser,
  ] = await Promise.all([
    getContests(currentPage, 10, search, org_id),
    getCurrentUser(),
  ]);

  const org = orgData!.organization;
  const contests = contestsData?.contests ?? [];
  const contestsPagination = contestsData?.pagination ?? { page: 1, total: 1 };
  const isAuthenticated = currentUser !== null;

  return (
    <DefaultLayout
      headerSecondaryNavItems={orgHeaderNav}
      headerOrganization={{ id: org.id, name: org.name }}
    >
      <Container size="lg" py="lg">
        {contestsError ? (
          <ErrorDisplay error={contestsError} />
        ) : (
          <OrgContestsTab
            contests={contests}
            pagination={contestsPagination}
            org={org}
            isAuthenticated={isAuthenticated}
            search={search}
          />
        )}
      </Container>
    </DefaultLayout >
  );
};

export default OrgPage;
