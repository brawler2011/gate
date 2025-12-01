import { cache } from "react";
import { numberToLetters } from "@/lib/lib";
import {
  getContest,
  getContestProblem,
  getSubmissions,
} from "@/lib/actions";
import { HeaderWithSession } from "@/components/HeaderWithSession";
import { Metadata } from "next";
import { notFound } from "next/navigation";
import { Task } from "@/components/Task";
import { getCurrentUser } from "@/lib/auth";
import { getMyContestRole } from "@/lib/contest-role";

type Props = {
  params: Promise<{ contest_id: string; problem_id: string; userId: string; sortOrder: string; }>;
};

// Cache getContestProblem to avoid duplicate calls in generateMetadata and Page
const getCachedContestProblem = cache(
  (problemId: string, contestId: string) =>
    getContestProblem(problemId, contestId)
);

const generateMetadata = async (props: Props): Promise<Metadata> => {
  const params = await props.params;
  const problem = await getCachedContestProblem(
    params.problem_id,
    params.contest_id
  );

  if (!problem?.problem) {
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

  const [problemResponse, contestResponse, submissionsResponse, user] =
    await Promise.all([
      getCachedContestProblem(params.problem_id, params.contest_id),
      getContest(params.contest_id),
      getSubmissions({
        page: 1,
        pageSize: 20,
        contestId: params.contest_id,
        sortOrder: "desc",
      }),
      getCurrentUser(),
    ]);

  if (!problemResponse?.problem || !contestResponse?.contest) {
    notFound();
  }

  // Get contest role for permissions
  const contestRole = user ? await getMyContestRole(params.contest_id) : null;

  // Handle submissions - if null or error, use empty array
  // This can happen if user is not synced in backend DB yet
  const submissions = submissionsResponse.submissions || [];

  // Build WebSocket URL for real-time updates
  // Remove trailing slash if present to avoid double slashes
  const wsBaseUrl = (process.env.NEXT_PUBLIC_WS_core_URL || '').replace(/\/+$/, '');
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
      contestRole={contestRole}
      header={<HeaderWithSession />}
      wsUrl={wsUrl}
    />
  );
};

export { Page as default, generateMetadata };
