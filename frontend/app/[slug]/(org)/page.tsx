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
  const contestsPagination = contestsData?.pagination ?? {page: 1, total: 1};
  const isAuthenticated = currentUser !== null;

  let isMember = false;
  if (currentUser) {
    // The members endpoint returns the complete list, without server-side pagination.
    const {data: membersData, error: membersError, response: membersResponse} = await api.GET(
      "/organizations/{login}/members",
      {
        params: {path: {login: org.login}},
      },
    );
    if (membersError && membersResponse.status !== 403) {
      throw membersError;
    }
    isMember = membersData?.members.some((member) => member.user_id === currentUser.id) ?? false;
  }
  return {search, org, contestsError, contests, contestsPagination, isAuthenticated, isMember};
}) satisfies LoaderFunction;

const Page = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {search, org, contestsError, contests, contestsPagination, isAuthenticated, isMember} =
    data;
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
