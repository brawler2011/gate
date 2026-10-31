import {Container} from "@mantine/core";
import {useLoaderData, Navigate} from "react-router-dom";

import {ContestMonitorTable} from "@/components/contests";
import {api, unwrap} from "@/lib/api";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

import type {ReactNode} from "react";
import type {LoaderFunction, LoaderFunctionArgs} from "react-router-dom";

export const loader = (async ({params: routeParams}: LoaderFunctionArgs) => {
  const {slug, contestLogin} = routeParams as {slug: string; contestLogin: string};

  // Fetch contest data and scoreboard
  const [contestResponse, scoreboard, {data: me}] = await Promise.all([
    unwrap(
      api.GET("/organizations/{org_login}/contests/{contest_login}", {
        params: {path: {org_login: slug, contest_login: contestLogin}},
      }),
    ),
    unwrap(
      api.GET("/organizations/{org_login}/contests/{contest_login}/scoreboard", {
        params: {path: {org_login: slug, contest_login: contestLogin}},
      }),
    ),
    api.GET("/users/me"),
  ]);
  const user = me?.user ?? null;
  const contestRole = user ? await getMyContestRole(slug, contestLogin) : null;
  return {slug, contestLogin, contestResponse, scoreboard, user, contestRole};
}) satisfies LoaderFunction;

const Page = (): ReactNode => {
  const data = useLoaderData<typeof loader>();
  const {slug, contestLogin, contestResponse, scoreboard, user, contestRole} = data;
  let isManager = false;
  if (contestResponse?.contest) {
    const checker = new PermissionChecker(
      user,
      contestRole?.role ?? null,
      null,
      contestRole?.permissionsMask ?? null,
    );
    isManager = checker.canManageContest(contestResponse.contest);
    const hasStarted =
      !contestResponse.contest.start_time || new Date(contestResponse.contest.start_time) <= new Date();

    if (!checker.canViewMonitor(contestResponse.contest) || (!isManager && !hasStarted)) {
      return <Navigate to={`/${slug}/${contestLogin}`} replace />;
    }
  }
  return (
    <Container size="lg" py="md">
      {scoreboard && (
        <ContestMonitorTable
          contestId={contestResponse.contest.id}
          orgLogin={slug}
          contestLogin={contestLogin}
          initialScoreboard={scoreboard}
          startTime={contestResponse?.contest.start_time}
          endTime={contestResponse?.contest.end_time}
          isManager={isManager}
        />
      )}
    </Container>
  );
};

export default Page;
