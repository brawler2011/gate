import {redirect} from "next/navigation";
import {cache} from "react";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {Task} from "@/components/shared/Task";
import {api, unwrapAndCache} from "@/lib/api";
import {env} from "@/lib/env";
import {numberToLetters} from "@/lib/lib";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

import type {Metadata} from "next";
import type {ReactNode} from "react";

const getCachedContestProblem = cache(
  async (problemId: string, contestId: string) => {
    return api.getContestProblem({problemId, contestId});
  },
);

type Props = {
  params: Promise<{
    slug: string;
    contest_id: string;
    problem_id: string;
  }>;
};

export const generateMetadata = async (props: Props): Promise<Metadata> => {
  const params = await props.params;

  const [error, response] = await getCachedContestProblem(
    params.problem_id,
    params.contest_id,
  );

  if (error || !response) {
    return {
      title: "Ошибка загрузки задачи",
    };
  }

  const problemIndex = response.problem.position ?? 0;
  const problemLetter = numberToLetters(problemIndex);

  return {
    title: `${problemLetter}. ${response.problem.title}`,
    description: `${problemLetter}. ${response.problem.title}`,
  };
};

const Page = async (props: Props): Promise<ReactNode> => {
  const params = await props.params;

  // First get the user to filter submissions by their ID
  const [, me] = await api.getMe();
  const user = me?.user ?? null;

  const [
    [problemError, problemResponse],
    contestResponse,
    [, submissionsResponse],
  ] = await Promise.all([
    getCachedContestProblem(params.problem_id, params.contest_id),
    unwrapAndCache(api.getContest)({contestId: params.contest_id}),
    // Only fetch user's own submissions if authenticated
    user
      ? api.listContestSubmissions({
        userId: user.id,
        contestId: params.contest_id,
        problemId: params.problem_id,
        page: 1,
        pageSize: 5,
        sortOrder: "desc",
      })
      : Promise.resolve([
        null,
        {submissions: [], pagination: {page: 1, total: 0}, since: 0},
      ] as const),
  ]);

  if (problemError) {
    return <ErrorDisplay error={problemError} />;
  }

  if (!problemResponse?.problem || !contestResponse?.contest) {
    return (
      <ErrorDisplay
        error={{status: 404, message: "Задача или контест не найдены"}}
      />
    );
  }

  // Get contest role for permissions
  const contestRole = user ? await getMyContestRole(params.contest_id) : null;

  const checker = new PermissionChecker(
    user,
    contestRole?.role ?? null,
    null,
    contestRole?.permissionsMask ?? null,
  );
  const isManager = checker.canManageContest(contestResponse.contest);
  const hasStarted =
    !contestResponse.contest.start_time ||
    new Date(contestResponse.contest.start_time) <= new Date();

  if (!checker.canViewProblems(contestResponse.contest) || (!isManager && !hasStarted)) {
    redirect(`/${params.slug}/contests/${params.contest_id}`);
  }

  // Handle submissions - if null or error, use empty array
  // This can happen if user is not synced in backend DB yet
  const submissions = [...(submissionsResponse?.submissions || [])];

  // Build WebSocket URL for real-time updates
  const wsBaseUrl = env.getWebSocketUrl();
  const wsUrl = wsBaseUrl ? `${wsBaseUrl}/submissions` : undefined;

  return (
    <Task
      task={problemResponse.problem}
      contest={contestResponse.contest}
      tasks={contestResponse.problems || []}
      submissions={submissions}
      problemId={params.problem_id}
      contestId={params.contest_id}
      user={user}
      wsUrl={wsUrl}
      since={submissionsResponse?.since}
      isManager={isManager}
    />
  );
};

export default Page;
