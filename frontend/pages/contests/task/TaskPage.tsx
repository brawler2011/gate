import {useParams, Navigate} from "react-router-dom";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {QueryState} from "@/components/shared/QueryState";
import {Task} from "@/components/shared/Task";
import {useContestData} from "@/hooks/useContestData";
import {api} from "@/lib/api";
import {lettersToNumber, numberToLetters} from "@/lib/lib";
import {SUBMISSIONS_PATH, submissionsParams} from "@/lib/submissions";

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
  const problemQuery = api.useQuery(
    "get",
    "/contests/{contest_id}/problems/{problem_id}",
    {
      params: {
        path: {
          contest_id: contest.data?.response.contest.id ?? "",
          problem_id: targetProblem?.problem_id ?? "",
        },
      },
    },
    {enabled: Boolean(targetProblem && contest.data)},
  );
  const submissionsSelection = {orgLogin: slug, contestLogin, contestId: contest.data?.response.contest.id ?? '',
    userId: user?.id, problemId: targetProblem?.problem_id, page: 1, pageSize: 5, sortOrder: 'desc' as const};
  const submissionsQuery = api.useQuery('get', SUBMISSIONS_PATH, submissionsParams(submissionsSelection),
    {enabled: Boolean(user && targetProblem), meta: {selection: submissionsSelection}});
  if (!contest.data) {
    return <QueryState error={contest.error} />;
  }
  if (targetPosition <= 0 || !targetProblem) {
    return <ErrorDisplay error={{status: 404}} />;
  }
  if (!problemQuery.data) {
    return <QueryState error={problemQuery.error} />;
  }
  const {response: contestResponse} = contest.data;
  const problemResponse = problemQuery.data;
  const submissionsResponse = submissionsQuery.data;
  const isManager = contestResponse.contest.permissions.manage;
  if (!contestResponse.contest.permissions.view_problems) {
    return <Navigate to={`/${params.slug}/${params.contestLogin}`} replace />;
  }
  if (user && submissionsQuery.isPending) {
    return <QueryState />;
  }
  const submissions = [...(submissionsResponse?.submissions || [])];
  return (
    <>
      <title>{problemResponse.problem.title}</title>
      <Task
        task={problemResponse.problem}
        contest={contestResponse.contest}
        tasks={contestResponse.problems || []}
        submissions={submissions}
        problemId={targetProblem.problem_id}
        user={user}
        isManager={isManager}
      />
    </>
  );
};

export default Page;
