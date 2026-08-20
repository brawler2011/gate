import {Container} from "@mantine/core";
import {redirect} from "next/navigation";

import {OrgContestsTab} from "@/components/orgs/OrgContestsTab";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api, unwrapAndCache} from "@/lib/api";
import {parsePage} from "@/lib/lib";

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ org_id: string }>;
  searchParams: Promise<{ page?: string; search?: string }>;
};

export const metadata: Metadata = {
  title: "Контесты",
};

const OrgPage = async ({params, searchParams}: Props): Promise<ReactNode> => {
  const {org_id} = await params;
  const {page, search} = await searchParams;
  const currentPage = parsePage(page);
  if (!currentPage) {
    redirect(`/orgs/${org_id}`);
  }

  const orgData = await unwrapAndCache(api.getOrganization)({id: org_id});

  const [
    [contestsError, contestsData],
    [, me],
  ] = await Promise.all([
    api.listWorkshopContests({page: currentPage, pageSize: 10, search, organizationId: org_id}),
    api.getMe(),
  ]);

  const currentUser = me?.user ?? null;
  const org = orgData.organization;
  const contests = contestsData?.contests ?? [];
  const contestsPagination = contestsData?.pagination ?? {page: 1, total: 1};
  const isAuthenticated = currentUser !== null;

  return (
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
  );
};

export default OrgPage;
