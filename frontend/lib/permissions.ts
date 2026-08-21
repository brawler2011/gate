import type {ContestModel, ProblemModel, UserModel} from "@/contracts/core/v1";

/**
 * Contest role types
 * Hierarchy: owner > moderator > participant
 */
export type ContestRole = "owner" | "moderator" | "participant";

export type ContestRoleResponse = {
  role: ContestRole;
  permissionsMask?: number;
} | null;

type ContestRoleApiResponse = {
  role?: unknown;
  permissions_mask?: unknown;
};

const parseContestRoleResponse = (response: unknown): ContestRoleResponse => {
  if (!response || typeof response !== "object") {
    return null;
  }

  const data = response as ContestRoleApiResponse;
  if (data.role !== "owner" && data.role !== "moderator" && data.role !== "participant") {
    return null;
  }

  const parsed: Exclude<ContestRoleResponse, null> = {role: data.role};
  if (typeof data.permissions_mask === "number") {
    parsed.permissionsMask = data.permissions_mask;
  }

  return parsed;
};

/**
 * Get the current user's role in a specific contest
 *
 * @param orgLogin - Organization login
 * @param contestLogin - Contest login
 * @returns The user's role in the contest, or null if not a participant
 */
export const getMyContestRole = async (orgLogin: string, contestLogin: string): Promise<ContestRoleResponse> => {
  const {api} = await import("@/lib/api");
  const [error, response] = await api.getMyContestRole({orgLogin, contestLogin});
  if (error || !response) {
    // User is not a participant or not authenticated
    return null;
  }

  return parseContestRoleResponse(response);
};

export type OrgRole = "owner" | "admin" | "member";

const ORG_ROLE_HIERARCHY: Record<OrgRole, number> = {
  owner: 3,
  admin: 2,
  member: 1,
};

export class PermissionChecker {
  private user: UserModel | null;
  private contestRole: ContestRole | null;
  private orgRole: OrgRole | null;
  private permissionsMask: number | null;

  constructor(
    user: UserModel | null = null,
    contestRole: ContestRole | null = null,
    orgRole: OrgRole | null = null,
    permissionsMask: number | null = null
  ) {
    this.user = user;
    this.contestRole = contestRole;
    this.orgRole = orgRole;
    this.permissionsMask = permissionsMask;
  }

  isAuthenticated(): boolean {
    return this.user !== null && !!this.user.id;
  }

  isGlobalAdmin(): boolean {
    return this.user?.role === "admin";
  }

  isOrgAdmin(): boolean {
    if (this.isGlobalAdmin()) {
      return true;
    }
    if (!this.orgRole) {
      return false;
    }
    return ORG_ROLE_HIERARCHY[this.orgRole] >= ORG_ROLE_HIERARCHY["admin"];
  }

  // Contest permissions

  isContestOwner(contest: ContestModel): boolean {
    if (this.isGlobalAdmin() || this.isOrgAdmin()) {
      return true;
    }
    if (!this.user?.id) {
      return false;
    }
    if (contest.created_by && this.user.id === contest.created_by) {
      return true;
    }
    if (contest.owner?.id && this.user.id === contest.owner.id) {
      return true;
    }
    return this.contestRole === "owner";
  }

  isContestModerator(contest: ContestModel): boolean {
    if (this.isContestOwner(contest)) {
      return true;
    }
    return this.contestRole === "moderator";
  }

  isContestParticipant(contest: ContestModel): boolean {
    if (this.isContestModerator(contest)) {
      return true;
    }
    if (this.contestRole === "participant") {
      return true;
    }
    // In open public contests, authenticated users automatically have participant status
    if (contest.visibility === "public" && (contest.participation_mode ?? "open") === "open" && this.isAuthenticated()) {
      return true;
    }
    return false;
  }

  canViewContest(contest: ContestModel): boolean {
    if (this.isContestModerator(contest)) {
      return true;
    }

    if (contest.visibility === "public") {
      return true;
    }

    return this.isContestParticipant(contest);
  }

  canViewProblems(contest: ContestModel): boolean {
    if (this.isContestModerator(contest)) {
      return true;
    }

    const hasStarted = !contest.start_time || new Date(contest.start_time) <= new Date();
    if (!hasStarted) {
      return false;
    }

    if (contest.visibility === "public") {
      return true;
    }

    return this.isContestParticipant(contest);
  }

  canSubmitSolution(contest: ContestModel): boolean {
    if (this.isContestModerator(contest)) {
      return true;
    }

    if (!this.isAuthenticated()) {
      return false;
    }

    const hasStarted = !contest.start_time || new Date(contest.start_time) <= new Date();
    if (!hasStarted) {
      return false;
    }

    const hasFinished = contest.end_time ? new Date(contest.end_time) <= new Date() : false;
    if (!hasFinished) {
      return this.isContestParticipant(contest);
    }

    // Finished contest: Codeforces style upsolving
    const enableUpsolving = contest.enable_upsolving ?? true;
    if (enableUpsolving) {
      return this.canViewContest(contest);
    }

    return false;
  }

  canViewMySubmissions(contest: ContestModel): boolean {
    if (this.isContestModerator(contest)) {
      return true;
    }

    if (!this.isAuthenticated()) {
      return false;
    }

    return this.isContestParticipant(contest);
  }

  canViewAllSubmissions(contest: ContestModel): boolean {
    if (this.isContestModerator(contest)) {
      return true;
    }

    const hasStarted = !contest.start_time || new Date(contest.start_time) <= new Date();
    const scope = contest.submissions_list_scope ?? "moderator";

    switch (scope) {
      case "public":
        return this.canViewContest(contest) && hasStarted;
      case "participant":
        return this.isContestParticipant(contest) && hasStarted;
      case "moderator":
        return this.isContestModerator(contest);
      default:
        return this.isContestModerator(contest);
    }
  }

  canViewSubmissionDetails(contest: ContestModel): boolean {
    if (this.isContestModerator(contest)) {
      return true;
    }

    const hasStarted = !contest.start_time || new Date(contest.start_time) <= new Date();
    const scope = contest.submission_details_scope ?? "moderator";

    switch (scope) {
      case "public":
        return this.canViewContest(contest) && hasStarted;
      case "participant":
        return this.isContestParticipant(contest) && hasStarted;
      case "moderator":
        return this.isContestModerator(contest);
      default:
        return this.isContestModerator(contest);
    }
  }

  canViewMonitor(contest: ContestModel): boolean {
    if (this.isContestModerator(contest)) {
      return true;
    }

    const hasStarted = !contest.start_time || new Date(contest.start_time) <= new Date();
    if (!hasStarted) {
      return false;
    }

    const scope = contest.monitor_scope ?? "participant";
    switch (scope) {
      case "public":
        return this.canViewContest(contest);
      case "participant":
        return this.isContestParticipant(contest);
      case "moderator":
        return this.isContestModerator(contest);
      default:
        return this.isContestParticipant(contest);
    }
  }

  canManageContest(contest: ContestModel): boolean {
    return this.isContestModerator(contest);
  }

  canDeleteContest(contest: ContestModel): boolean {
    return this.isContestOwner(contest);
  }

  canManageContestParticipants(contest: ContestModel): boolean {
    return this.isContestModerator(contest);
  }

  canRejudgeSubmissions(contest: ContestModel): boolean {
    return this.isContestModerator(contest);
  }

  // Problem permissions

  canViewProblem(problem: ProblemModel): boolean {
    if (this.isGlobalAdmin() || this.isOrgAdmin()) {
      return true;
    }
    if (problem.created_by && this.user?.id === problem.created_by) {
      return true;
    }

    return problem.visibility === "public" || (!problem.is_private && problem.visibility !== "private");
  }

  canEditProblem(problem: ProblemModel): boolean {
    if (this.isGlobalAdmin() || this.isOrgAdmin()) {
      return true;
    }
    if (problem.created_by && this.user?.id === problem.created_by) {
      return true;
    }

    return false;
  }

  canDeleteProblem(problem: ProblemModel): boolean {
    return this.canEditProblem(problem);
  }

  // User permissions

  canEditUser(userId: string): boolean {
    if (!this.isAuthenticated()) {
      return false;
    }

    // User can edit themselves
    if (this.user?.id === userId) {
      return true;
    }

    // Global admin can edit any user
    return this.isGlobalAdmin();
  }

  canDeleteUser(_userId: string): boolean {
    if (!this.isAuthenticated()) {
      return false;
    }

    // Only admin can delete users
    return this.isGlobalAdmin();
  }

  // Org permissions

  canManageOrgMembers(): boolean {
    return this.isOrgAdmin();
  }

  canCreateTeam(): boolean {
    return this.canManageOrgMembers();
  }

  canDeleteOrg(): boolean {
    if (this.isGlobalAdmin()) {
      return true;
    }
    return this.orgRole === "owner";
  }

  canManageTeamMembers(): boolean {
    return this.canManageOrgMembers();
  }
}

export const canManageOrgMembers = async (orgLogin: string): Promise<boolean> => {
  const {api} = await import("@/lib/api");
  const [, me] = await api.getMe();
  const currentUser = me?.user ?? null;
  if (!currentUser) {
    return false;
  }
  if (currentUser.role === "admin") {
    return true;
  }

  let page = 1;
  const pageSize = 100;
  while (page <= 20) {
    const [error, data] = await api.listOrganizationMembers({login: orgLogin, page, pageSize});
    if (error || !data || !data.members) {
      return false;
    }

    const member = data.members.find((m) => m.user_id === currentUser.id);
    if (member) {
      return member.role === "owner" || member.role === "admin";
    }

    const total = data.pagination?.total ?? 0;
    if (page * pageSize >= total || data.members.length < pageSize) {
      break;
    }
    page++;
  }

  return false;
};
