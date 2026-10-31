import {Container} from "@mantine/core";

import {api, unwrap} from "@/lib/api";
import {usePageQuery, PageQueryState} from "@/lib/pageQuery";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";
import {redirect} from "@/lib/routeErrors";

import {DraftsClient} from "./DraftsClient";

import type {ReactNode} from "react";

const loadPage = async ({params}: PageProps) => {
  const {slug, contestLogin} = params;

  const [contestResponse, [, me]] = await Promise.all([
    unwrap(api.getContest)(slug, contestLogin),
    api.getMe(),
  ]);

  if (!contestResponse?.contest) {
    throw {status: 404, message: "Контест не найден"};
  }

  const user = me?.user ?? null;
  if (!user?.id) {
    redirect(`/${slug}/${contestLogin}`);
  }

  const contestRole = await getMyContestRole(slug, contestLogin);

  const checker = new PermissionChecker(
    user,
    contestRole?.role ?? null,
    null,
    contestRole?.permissionsMask ?? null,
  );

  const isManager = checker.canManageContest(contestResponse.contest);
  const hasStarted =
    !contestResponse.contest.start_time || new Date(contestResponse.contest.start_time) <= new Date();

  const draftsDisabled = contestResponse.contest.enable_drafts === false;
  if (!checker.canViewProblems(contestResponse.contest) || (!isManager && (!hasStarted || draftsDisabled))) {
    redirect(`/${slug}/${contestLogin}`);
  }

  const isContestEnded = contestResponse.contest.end_time
    ? new Date(contestResponse.contest.end_time) <= new Date()
    : false;

  const [, draftsResponse] = await api.listContestDrafts(slug, contestLogin, {
    page: 1, pageSize: 50
  });
  return {
    slug,
    contestLogin,
    contestResponse,
    me,
    user,
    contestRole,
    checker,
    isManager,
    hasStarted,
    draftsDisabled,
    isContestEnded,
    draftsResponse,
  };
};

type PageProps = {
  params: { slug: string; contestLogin: string };
};

const Page = ({params}: PageProps): ReactNode => {
  const routeQuery = usePageQuery(["frontend/app/[slug]/[contestLogin]/drafts/page.tsx", params], () =>
    loadPage({params}),
  );
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
  }
  const {contestResponse, isManager, isContestEnded, draftsResponse} = routeQuery.data;
  const initialDrafts = draftsResponse?.drafts || [];
  return (
    <Container size="lg" pb={{base: "md", sm: "lg", md: "xl"}} pt="md">
      <DraftsClient
        contest={contestResponse.contest}
        isManager={isManager}
        isContestEnded={isContestEnded}
        initialDrafts={initialDrafts}
      />
    </Container>
  );
};

export default Page;
