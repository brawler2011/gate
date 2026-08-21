import {Container} from "@mantine/core";
import {notFound, redirect} from "next/navigation";

import {OrgContestsTab} from "@/components/orgs/OrgContestsTab";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api, unwrapAndCache} from "@/lib/api";
import {parsePage} from "@/lib/lib";

import type {Metadata} from "next";
import type {ReactNode} from "react";

type Props = {
  params: Promise<{ slug: string }>;
  searchParams: Promise<{ page?: string; search?: string }>;
};

const getOrganization = unwrapAndCache(api.getOrganization);

export const generateMetadata = async ({params}: Props): Promise<Metadata> => {
  const {slug} = await params;
  let decoded = "";
  try {
    decoded = decodeURIComponent(slug);
  } catch {
    notFound();
  }

  const [orgError, orgData] = await api.getOrganization({login: decoded});
  if (orgError || !orgData) {
    return {title: "Организация"};
  }

  return {title: orgData.organization.name};
};

const Page = async ({params, searchParams}: Props): Promise<ReactNode> => {
  const {slug} = await params;
  const {page, search} = await searchParams;

  let decoded = "";
  try {
    decoded = decodeURIComponent(slug);
  } catch {
    notFound();
  }

  const currentPage = parsePage(page);
  if (!currentPage) {
    redirect(`/${decoded}`);
  }

  const orgData = await getOrganization({login: decoded});
  const org = orgData.organization;

  const [
    [contestsError, contestsData],
    [, me],
  ] = await Promise.all([
    api.listWorkshopContests({page: currentPage, pageSize: 10, search, organizationId: org.id}),
    api.getMe(),
  ]);

  const currentUser = me?.user ?? null;
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

export default Page;
