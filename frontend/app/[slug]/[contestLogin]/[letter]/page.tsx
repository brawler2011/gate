import {useParams, Navigate} from "react-router-dom";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {QueryState} from "@/components/shared/QueryState";
import {Task} from "@/components/shared/Task";
import {useGetContestProblem, useListContestSubmissions} from "@/contracts/core/v1/api";
import {useContestData} from "@/hooks/useContestData";
import {env} from "@/lib/env";
import {lettersToNumber, numberToLetters} from "@/lib/lib";
import {PermissionChecker} from "@/lib/permissions";

import type {ReactNode} from "react";

type Props = {
  params: {
    slug: string;
    contestLogin: string;
    letter: string;
  };
};

const Page = (): ReactNode => {
  const params = useParams() as Props["params"];
  const {slug, contestLogin, letter} = params;
  const contest = useContestData(slug, contestLogin);
  const user = contest.data?.user ?? null;
  const targetPosition = lettersToNumber(letter);
  const targetProblem = contest.data?.response.problems?.find(
    (problem) => problem.position === targetPosition || numberToLetters(problem.position) === letter,
  );
  const problemQuery = useGetContestProblem(slug, contestLogin, targetProblem?.problem_id ?? '', {
    query: {enabled: Boolean(targetProblem)},
  });
  const submissionsQuery = useListContestSubmissions(slug, contestLogin, {
    userId: user?.id, problemId: targetProblem?.problem_id, page: 1, pageSize: 5, sortOrder: 'desc',
  }, {query: {enabled: Boolean(user && targetProblem)}});
  if (!contest.data) {
    return <QueryState error={contest.error} />;
  }
  if (targetPosition <= 0 || !targetProblem) {
    return <ErrorDisplay error={{status: 404, message: 'Задача не найдена'}} />;
  }
  if (!problemQuery.data) {
    return <QueryState error={problemQuery.error} />;
  }
  const {response: contestResponse, contestRole} = contest.data;
  const problemResponse = problemQuery.data;
  const submissionsResponse = submissionsQuery.data;
  const checker = new PermissionChecker(
    user,
    contestRole?.role ?? null,
    null,
    contestRole?.permissionsMask ?? null,
  );
  const isManager = checker.canManageContest(contestResponse.contest);
  const hasStarted =
    !contestResponse.contest.start_time || new Date(contestResponse.contest.start_time) <= new Date();
  if (!checker.canViewProblems(contestResponse.contest) || (!isManager && !hasStarted)) {
    return <Navigate to={`/${params.slug}/${params.contestLogin}`} replace />;
  }
  const submissions = [...(submissionsResponse?.submissions || [])];
  const wsBaseUrl = env.getWebSocketUrl();
  const wsUrl = wsBaseUrl ? `${wsBaseUrl}/submissions` : undefined;
  return (
    <Task
      task={problemResponse.problem}
      contest={contestResponse.contest}
      tasks={contestResponse.problems || []}
      submissions={submissions}
      problemId={targetProblem.problem_id}
      contestId={contestResponse.contest.id}
      user={user}
      wsUrl={wsUrl}
      since={submissionsResponse?.since}
      isManager={isManager}
    />
  );
};

export default Page;
