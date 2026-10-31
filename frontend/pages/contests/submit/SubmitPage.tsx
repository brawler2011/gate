import {Container} from "@mantine/core";
import {useParams, Navigate} from "react-router-dom";

import {QueryState} from "@/components/shared/QueryState";
import {useContestData} from "@/hooks/useContestData";
import {PermissionChecker} from "@/lib/permissions";

import {SubmitSubmissionClient} from "./SubmitSubmissionClient";

import type {ReactNode} from "react";

const Page = (): ReactNode => {
  const params = useParams() as {slug: string; contestLogin: string};
  const {slug, contestLogin} = params;
  const routeQuery = useContestData(slug, contestLogin);
  if (!routeQuery.data) {
    return <QueryState error={routeQuery.error} />;
  }
  const {response, user, contestRole} = routeQuery.data;
  const checker = new PermissionChecker(
    user,
    contestRole?.role ?? null,
    null,
    contestRole?.permissionsMask ?? null,
  );
  const isManager = checker.canManageContest(response.contest);
  const hasStarted = !response.contest.start_time || new Date(response.contest.start_time) <= new Date();
  if (!checker.canSubmitSolution(response.contest) || (!isManager && !hasStarted)) {
    return <Navigate to={`/${slug}/${contestLogin}`} replace />;
  }
  return (
    <Container size="lg" pb={{base: "md", sm: "lg", md: "xl"}}>
      <SubmitSubmissionClient contest={response.contest} problems={response.problems || []} user={user} />
    </Container>
  );
};

export default Page;
