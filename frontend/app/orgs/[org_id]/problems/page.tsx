import {Container} from "@mantine/core";
import {redirect} from "next/navigation";

import {OrgProblemsTab} from "@/components/orgs/OrgProblemsTab";
import {DefaultLayout} from "@/components/shared";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api} from "@/lib/api";
import {unwrapAndCache} from "@/lib/api2";
import {parsePage} from "@/lib/lib2";
import {buildOrgHeaderNav} from "@/lib/org-header-nav";
import {canManageOrgMembers} from "@/lib/org-permissions";

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ org_id: string }>;
  searchParams: Promise<{ page?: string }>;
};

export const metadata: Metadata = {
  title: "Задачи",
};

const OrgProblemsPage = async ({params, searchParams}: Props): Promise<ReactNode> => {
  const {org_id} = await params;
  const {page} = await searchParams;
  const currentPage = parsePage(page);
  if (!currentPage) {
    redirect(`/orgs/${org_id}/problems`);
  }
  const showMembersTab = await canManageOrgMembers(org_id);
  const orgHeaderNav = buildOrgHeaderNav({
    orgId: org_id,
    activeTab: "problems",
    showMembersTab,
  });

  const orgData = await unwrapAndCache(api.getOrganization)({id: org_id});

  const [
    [problemsError, problemsData],
    [, me],
  ] = await Promise.all([
    api.listProblems({page: currentPage, pageSize: 20, organizationId: org_id}),
    api.getMe(),
  ]);

  const currentUser = me?.user ?? null;
  const org = orgData.organization;
  const problems = problemsData?.problems ?? [];
  const problemsPagination = problemsData?.pagination ?? {page: 1, total: 1};
  const isAuthenticated = currentUser !== null;

  return (
    <DefaultLayout
      headerSecondaryNavItems={orgHeaderNav}
      headerOrganization={{id: org.id, name: org.name}}
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
