import {redirect} from "next/navigation";
import {cache} from "react";

import {ErrorDisplay} from "@/components/shared/ErrorDisplay";
import {Task} from "@/components/shared/Task";
import {api, unwrapAndCache} from "@/lib/api";
import {env} from "@/lib/env";
import {lettersToNumber, numberToLetters} from "@/lib/lib";
import {getMyContestRole, PermissionChecker} from "@/lib/permissions";

import type {Metadata} from "next";
import type {ReactNode} from "react";

const getCachedContestProblem = cache(
  async (orgLogin: string, contestLogin: string, problemId: string) => {
    return api.getContestProblem({orgLogin, contestLogin, problemId});
  },
);

type Props = {
  params: Promise<{
    slug: string;
    contestLogin: string;
    letter: string;
  }>;
};

export const generateMetadata = async (props: Props): Promise<Metadata> => {
  const params = await props.params;

  const targetPosition = lettersToNumber(params.letter);
  if (targetPosition <= 0) {
    return {
      title: "Задача не найдена",
    };
  }

  const contestResponse = await unwrapAndCache(api.getContest)({
    orgLogin: params.slug,
    contestLogin: params.contestLogin,
  });

  const targetProblem = contestResponse?.problems?.find(
    (p) => p.position === targetPosition || numberToLetters(p.position) === params.letter,
  );

  if (!targetProblem) {
    return {
      title: "Задача не найдена",
    };
  }

  const [error, response] = await getCachedContestProblem(
    params.slug,
    params.contestLogin,
    targetProblem.problem_id,
  );

  if (error || !response) {
    return {
      title: "Ошибка загрузки задачи",
    };
  }

  const problemIndex = response.problem.position ?? targetPosition;
  const problemLetter = numberToLetters(problemIndex);

  return {
    title: `${problemLetter}. ${response.problem.title}`,
    description: `${problemLetter}. ${response.problem.title}`,
  };
};

const Page = async (props: Props): Promise<ReactNode> => {
  const params = await props.params;

  const targetPosition = lettersToNumber(params.letter);
  if (targetPosition <= 0) {
    return (
      <ErrorDisplay
        error={{status: 404, message: "Задача не найдена"}}
      />
    );
  }

  // First get the user to filter submissions by their ID
  const [, me] = await api.getMe();
  const user = me?.user ?? null;

  const contestResponse = await unwrapAndCache(api.getContest)({
    orgLogin: params.slug,
    contestLogin: params.contestLogin,
  });

  if (!contestResponse?.contest) {
    return (
      <ErrorDisplay
        error={{status: 404, message: "Задача или контест не найдены"}}
      />
    );
  }

  const targetProblem = contestResponse.problems?.find(
    (p) => p.position === targetPosition || numberToLetters(p.position) === params.letter,
  );

  if (!targetProblem) {
    return (
      <ErrorDisplay
        error={{status: 404, message: "Задача или контест не найдены"}}
      />
    );
  }

  const [
    [problemError, problemResponse],
    [, submissionsResponse],
  ] = await Promise.all([
    getCachedContestProblem(params.slug, params.contestLogin, targetProblem.problem_id),
    // Only fetch user's own submissions if authenticated
    user
      ? api.listContestSubmissions({
        orgLogin: params.slug,
        contestLogin: params.contestLogin,
        userId: user.id,
        problemId: targetProblem.problem_id,
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

  if (!problemResponse?.problem) {
    return (
      <ErrorDisplay
        error={{status: 404, message: "Задача или контест не найдены"}}
      />
    );
  }

  // Get contest role for permissions
  const contestRole = user ? await getMyContestRole(params.slug, params.contestLogin) : null;

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
    redirect(`/${params.slug}/${params.contestLogin}`);
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
