import { ErrorDisplay } from "@/components/shared/ErrorDisplay";
import { HeaderWithSession } from "@/components/shared/HeaderWithSession";
import { Task } from "@/components/shared/Task";
import { getContest, getContestProblem, getMySubmissions } from "@/lib/actions";
import { getCurrentUser } from "@/lib/auth";
import { buildContestHeaderNav } from "@/lib/contest-header-nav";
import { getMyContestRole } from "@/lib/contest-role";
import { PermissionChecker } from "@/lib/permissions";
import { numberToLetters } from "@/lib/lib";
import { Metadata } from "next";
import { redirect } from "next/navigation";
import { cache } from "react";

type Props = {
  params: Promise<{
    contest_id: string;
    problem_id: string;
    userId: string;
    sortOrder: string;
  }>;
};

// Cache getContestProblem to avoid duplicate calls in generateMetadata and Page
const getCachedContestProblem = cache((problemId: string, contestId: string) =>
  getContestProblem(problemId, contestId),
);

const generateMetadata = async (props: Props): Promise<Metadata> => {
  const params = await props.params;
  const [error, problem] = await getCachedContestProblem(
    params.problem_id,
    params.contest_id,
  );

  if (error || !problem?.problem) {
    return {
      title: "Что-то пошло не так!",
    };
  }

  return {
    title: `${numberToLetters(problem.problem.position)}. ${
      problem.problem.title
    }`,
    description: problem.problem.legend_html,
  };
};

const Page = async (props: Props) => {
  const params = await props.params;

  // First get the user to filter submissions by their ID
  const user = await getCurrentUser();

  const [
    [problemError, problemResponse],
    [contestError, contestResponse],
    [, submissionsResponse],
  ] = await Promise.all([
    getCachedContestProblem(params.problem_id, params.contest_id),
    getContest(params.contest_id),
    // Only fetch user's own submissions if authenticated
    user
      ? getMySubmissions({
          userId: user.id,
          contestId: params.contest_id,
          problemId: params.problem_id,
          page: 1,
          pageSize: 5,
          sortOrder: "desc",
        })
      : Promise.resolve([
          null,
          { submissions: [], pagination: { page: 1, total: 0 }, since: 0 },
        ] as const),
  ]);

  if (problemError) return <ErrorDisplay error={problemError} />;
  if (contestError) return <ErrorDisplay error={contestError} />;

  if (!problemResponse?.problem || !contestResponse?.contest) {
    return (
      <ErrorDisplay
        error={{ status: 404, message: "Задача или контест не найдены" }}
      />
    );
  }

  // Get contest role for permissions
  const contestRole = user ? await getMyContestRole(params.contest_id) : null;

  const checker = new PermissionChecker(user, contestRole?.role ?? null);
  const isManager = checker.canManageContest(contestResponse.contest);
  const hasStarted = !contestResponse.contest.start_time || new Date(contestResponse.contest.start_time) <= new Date();

  if (!isManager && !hasStarted) {
    redirect(`/contests/${params.contest_id}`);
  }

  const contestHeaderNav = buildContestHeaderNav({
    contest: contestResponse.contest,
    user,
    contestRole,
    activeTab: "tasks",
  });

  // Handle submissions - if null or error, use empty array
  // This can happen if user is not synced in backend DB yet
  const submissions = [...(submissionsResponse?.submissions || [])];

  // Build WebSocket URL for real-time updates
  // Remove trailing slash if present to avoid double slashes
  const wsBaseUrl = (process.env.WEBSOCKET_URL || "").replace(/\/+$/, "");
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
      header={
        <HeaderWithSession
          secondaryNavItems={contestHeaderNav}
          organizationId={contestResponse.contest.organization_id}
        />
      }
      wsUrl={wsUrl}
      since={submissionsResponse?.since}
    />
  );
};

export { Page as default, generateMetadata };
