import {Container} from "@mantine/core";

import {useContestData} from "@/hooks/useContestData";
import {PageQueryState} from "@/lib/pageQuery";
import {PermissionChecker} from "@/lib/permissions";
import {redirect} from "@/lib/routeErrors";

import {SubmitSubmissionClient} from "./SubmitSubmissionClient";

import type {ReactNode} from "react";

type PageProps = {
  params: { slug: string; contestLogin: string };
};

const Page = ({params}: PageProps): ReactNode => {
  const {slug, contestLogin} = params;
  const routeQuery = useContestData(slug, contestLogin);
  if (!routeQuery.data) {
    return <PageQueryState query={routeQuery} />;
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
    redirect(`/${slug}/${contestLogin}`);
  }
  return (
    <Container size="lg" pb={{base: "md", sm: "lg", md: "xl"}}>
      <SubmitSubmissionClient contest={response.contest} problems={response.problems || []} user={user} />
    </Container>
  );
};

export default Page;
