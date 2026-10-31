import {Container} from "@mantine/core";

import {ContestMonitorTable} from "@/components/contests";
import {api, unwrap} from "@/lib/api";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";
import {redirect} from "@/lib/routeErrors";

import type {ReactNode} from "react";

const loadPage = async ({params}: PageProps) => {
  const {slug, contestLogin} = params;

  // Fetch contest data and scoreboard
  const contestResponse = await unwrap(api.getContest)(slug, contestLogin);
  const scoreboard = await unwrap(api.getContestScoreboard)(slug, contestLogin, {});
  const [, me] = await api.getMe();
  const user = me?.user ?? null;
  const contestRole = user ? await getMyContestRole(slug, contestLogin) : null;
  return {
    slug,
    contestLogin,
    contestResponse,
    scoreboard,
    me,
    user,
    contestRole,
  };
};

type PageProps = {
  params: { slug: string; contestLogin: string };
};

const Page = ({params}: PageProps): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/[slug]/[contestLogin]/monitor/page.tsx", params], () =>
    loadPage({params}),
  );
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {slug, contestLogin, contestResponse, scoreboard, user, contestRole} = routeQuery.data;
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
      redirect(`/${slug}/${contestLogin}`);
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
