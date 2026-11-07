import {Container} from "@mantine/core";
import {useLoaderData, redirect} from "react-router-dom";

import {OrgContestsTab} from "@/components/orgs/OrgContestsTab";
import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {api, unwrap} from "@/lib/api";
import {parsePage} from "@/lib/lib";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams, request}: LoaderFunctionArgs) => {
  const searchParams = Object.fromEntries(new URL(request.url).searchParams);
  const {slug} = routeParams as {slug: string};
  const {page, search} = searchParams;

  const decoded = slug;

  const currentPage = parsePage(page);
  if (!currentPage) {
    throw redirect(`/${decoded}`);
  }

  const orgData = await unwrap(
    api.GET("/organizations/{login}", {
      params: {path: {login: decoded}},
    }),
  );
  const org = orgData.organization;

  const [{data: contestsData, error: contestsErr, response: contestsResponse}, {data: me}] = await Promise.all([
    api.GET("/workshop/contests", {
      params: {
        query: {
          page: currentPage,
          pageSize: 10,
          search,
          organization_id: org.id,
        },
      },
    }),
    api.GET("/users/me"),
  ]);

  const contestsError = contestsErr
    ? {
      status: contestsResponse.status,
      message: contestsErr.message || contestsResponse.statusText,
    }
    : null;

  const currentUser = me?.user ?? null;
  const contests = contestsData?.contests ?? [];
  const contestsPagination = contestsData?.pagination ?? {page_size: 50, total_items: 0, page: 1, total_pages: 0};
  const isAuthenticated = currentUser !== null;

  let isMember = false;
  if (currentUser) {
    let memberPage = 1;
    while (true) {
      const {data: membersData, error: membersError, response: membersResponse} = await api.GET(
        "/organizations/{login}/members",
        {params: {path: {login: org.login}, query: {page: memberPage, pageSize: 100}}},
      );
      if (membersError && membersResponse.status !== 403) {
        throw membersError;
      }
      isMember = membersData?.members.some((member) => member.user_id === currentUser.id) ?? false;
      if (isMember || !membersData || memberPage >= membersData.pagination.total_pages) {
        break;
      }
      memberPage++;
    }
  }
  return {search, org, contestsError, contests, contestsPagination, isAuthenticated, isMember};
}) satisfies LoaderFunction;

const Page = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {search, org, contestsError, contests, contestsPagination, isAuthenticated, isMember} =
    data;
  return (
    <Container size="lg" py="lg">
      <title>{org.name}</title>
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
