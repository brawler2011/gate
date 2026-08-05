import { Container } from "@mantine/core";
import { notFound } from "next/navigation";

import { OrgProblemsTab } from "@/components/orgs/OrgProblemsTab";
import { DefaultLayout } from "@/components/shared";
import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { getProblems, getOrganization } from "@/lib/actions";
import { getCurrentUser } from "@/lib/auth";
import { buildOrgHeaderNav } from "@/lib/org-header-nav";
import { canManageOrgMembers } from "@/lib/org-permissions";

type Props = {
  params: Promise<{ org_id: string }>;
  searchParams: Promise<{ page?: string }>;
};

const OrgProblemsPage = async ({ params, searchParams }: Props) => {
  const { org_id } = await params;
  const { page } = await searchParams;
  const showMembersTab = await canManageOrgMembers(org_id);
  const orgHeaderNav = buildOrgHeaderNav({
    orgId: org_id,
    activeTab: "problems",
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
    [problemsError, problemsData],
    currentUser,
  ] = await Promise.all([
    getProblems(currentPage, 20, undefined, undefined, undefined, org_id),
    getCurrentUser(),
  ]);

  const org = orgData!.organization;
  const problems = problemsData?.problems ?? [];
  const problemsPagination = problemsData?.pagination ?? { page: 1, total: 1 };
  const isAuthenticated = currentUser !== null;

  return (
    <DefaultLayout
      headerSecondaryNavItems={orgHeaderNav}
      headerOrganization={{ id: org.id, name: org.name }}
    >
      <Container size="lg" py="lg">
        {problemsError ? (
          <ErrorDisplay error={problemsError} />
        ) : (
          <OrgProblemsTab
            problems={problems}
            pagination={problemsPagination}
            org={org}
            isAuthenticated={isAuthenticated}
          />
        )}
      </Container>
    </DefaultLayout>
  );
};

export default OrgProblemsPage;
