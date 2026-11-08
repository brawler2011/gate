import type {ContestModel, GetMyContestRoleResponseModel, ProblemModel} from "@/contracts/core/v1/schema";

export type ContestRoleResponse = GetMyContestRoleResponseModel | null;
export const parseContestRoleResponse = (response: unknown): ContestRoleResponse => {
  if (!response || typeof response !== "object") {
    return null;
  }
  const data = response as Record<string, unknown>;
  if (typeof data.role !== "string" || !Array.isArray(data.capabilities)
    || !data.capabilities.every((value) => typeof value === "string")) {
    return null;
  }
  return {role: data.role, capabilities: data.capabilities};
};

export const getMyContestRole = async (orgLogin: string, contestLogin: string): Promise<ContestRoleResponse> => {
  const {api} = await import("@/lib/api");
  const {data: response, error} = await api.GET(
    "/organizations/{org_login}/contests/{contest_login}/my-role",
    {
      params: {
        path: {org_login: orgLogin, contest_login: contestLogin},
      },
    },
  );
  if (error || !response) {
    // User is not a participant or not authenticated
    return null;
  }

  return parseContestRoleResponse(response);
};

/** All resource decisions come from the server's policy projection. */
export class PermissionChecker {
  canViewContest(contest: ContestModel): boolean {
    return contest.capabilities.includes("view");
  }
  canViewProblems(contest: ContestModel): boolean {
    return contest.capabilities.includes("view_statements");
  }
  canSubmitSolution(contest: ContestModel): boolean {
    return contest.capabilities.includes("submit");
  }
  canViewMySubmissions(contest: ContestModel): boolean {
    return contest.capabilities.includes("list_own_submissions");
  }
  canViewAllSubmissions(contest: ContestModel): boolean {
    return contest.capabilities.includes("list_submissions");
  }
  canViewSubmissionDetails(contest: ContestModel): boolean {
    return contest.capabilities.includes("view_submission_details");
  }
  canViewMonitor(contest: ContestModel): boolean {
    return contest.capabilities.includes("view_scoreboard");
  }
  canManageContest(contest: ContestModel): boolean {
    return contest.capabilities.includes("manage");
  }
  canDeleteContest(contest: ContestModel): boolean {
    return contest.capabilities.includes("delete");
  }
  canManageContestParticipants(contest: ContestModel): boolean {
    return contest.capabilities.includes("manage_members");
  }
  canRejudgeSubmissions(contest: ContestModel): boolean {
    return contest.capabilities.includes("manage");
  }
  canViewProblem(problem: ProblemModel): boolean {
    return problem.capabilities.includes("view");
  }
  canEditProblem(problem: ProblemModel): boolean {
    return problem.capabilities.includes("edit");
  }
  canDeleteProblem(problem: ProblemModel): boolean {
    return problem.capabilities.includes("delete");
  }
}

export const canManageOrgMembers = async (orgLogin: string): Promise<boolean> => {
  const {api} = await import("@/lib/api");
  const {data, error} = await api.GET("/organizations/{login}", {params: {path: {login: orgLogin}}});
  return !error && !!data?.organization.capabilities.includes("manage_members");
};
