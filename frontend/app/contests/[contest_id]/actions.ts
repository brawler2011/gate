"use server";

import { Call } from "@/lib/api";

type CreateTaskParams = {
  contest_id: string;
  problem_id: string;
};

export const CreateTask = async ({
  contest_id,
  problem_id,
}: CreateTaskParams) => {
  if (!contest_id || !problem_id) {
    console.error("Missing contest_id or problem_id", { contest_id, problem_id });
    return [{ status: 400, message: "Missing contest_id or problem_id" }, null] as const;
  }

  console.log("🚀 Creating task with:", { contest_id, problem_id });

  const [error, response] = await Call((client) =>
    client.default.createContestProblem({ contestId: contest_id, problemId: problem_id })
  );

  if (error) {
    console.error("❌ Failed to create task:", error);
    return [error, null] as const;
  }

  console.log("✅ Task created:", response);
  return [null, response?.id || null] as const;
};

export const fetchProblems = async (title: string) => {
  const [error, response] = await Call((client) =>
    client.default.listProblems({ page: 1, pageSize: 20, search: title })
  );

  if (error) {
    console.error("Failed to fetch problems:", error);
    return [];
  }

  return response?.problems || [];
};
