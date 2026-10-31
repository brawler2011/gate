import {Container} from "@mantine/core";

import {OrgContestsTab} from "@/components/orgs/OrgContestsTab";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api, unwrap} from "@/lib/api";
import {parsePage} from "@/lib/lib";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {notFound, redirect} from "@/lib/routeErrors";

import type {ReactNode} from "react";

const loadPage = async ({params, searchParams}: Props) => {
  const {slug} = params;
  const {page, search} = searchParams;

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

  const orgData = await getOrganization(decoded);
  const org = orgData.organization;

  const [[contestsError, contestsData], [, me]] = await Promise.all([
    api.listWorkshopContests({
      page: currentPage,
      pageSize: 10,
      search,
      organization_id: org.id
    }),
    api.getMe(),
  ]);

  const currentUser = me?.user ?? null;
  const contests = contestsData?.contests ?? [];
  const contestsPagination = contestsData?.pagination ?? {page: 1, total: 1};
  const isAuthenticated = currentUser !== null;

  let isMember = false;
  if (currentUser) {
    // The members endpoint returns the complete list, without server-side pagination.
    const [membersError, membersData] = await api.listOrganizationMembers(org.login, {});
    if (membersError && membersError.status !== 403) {
      throw membersError;
    }
    isMember = membersData?.members.some((member) => member.user_id === currentUser.id) ?? false;
  }
  return {
    slug,
    page,
    search,
    decoded,
    currentPage,
    orgData,
    org,
    contestsError,
    contestsData,
    me,
    currentUser,
    contests,
    contestsPagination,
    isAuthenticated,
    isMember,
  };
};

type Props = {
  params: { slug: string };
  searchParams: { page?: string; search?: string };
};

const getOrganization = unwrap(api.getOrganization);

const Page = ({params, searchParams}: Props): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/[slug]/(org)/page.tsx", params, searchParams], () =>
    loadPage({params, searchParams}),
  );
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {search, org, contestsError, contests, contestsPagination, isAuthenticated, isMember} =
    routeQuery.data;
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
          isMember={isMember}
          search={search}
        />
      )}
    </Container>
  );
};

export default Page;
