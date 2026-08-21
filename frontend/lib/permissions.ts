import {cache} from "react";

import {api} from "@/lib/api";

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
export const getMyContestRole = cache(async (orgLogin: string, contestLogin: string): Promise<ContestRoleResponse> => {
  const [error, response] = await api.getMyContestRole({orgLogin, contestLogin});
  if (error || !response) {
    // User is not a participant or not authenticated
    return null;
  }
  
  return parseContestRoleResponse(response);
});

/**
 * Permission checker utilities for frontend
 * These are client-side checks based on available data
 * Backend always performs authoritative permission checks
 */

type ContestScope = "owner" | "moderator" | "participant";

// Иерархия ролей: owner > moderator > participant
const ROLE_HIERARCHY: Record<ContestRole, number> = {
  owner: 3,
  moderator: 2,
  participant: 1,
};

export type OrgRole = 'owner' | 'admin' | 'member';

const ORG_ROLE_HIERARCHY: Record<OrgRole, number> = {
  owner: 3,
  admin: 2,
  member: 1,
};

/**
 * Check if user's role meets the required scope
 * @param userRole - User's role in the contest
 * @param requiredScope - Required scope/permission level
 * @returns true if user has required role or higher
 */
const hasRequiredRole = (userRole: ContestRole, requiredScope: ContestScope): boolean => {
  return ROLE_HIERARCHY[userRole] >= ROLE_HIERARCHY[requiredScope];
};

const ContestPermissionMasks = {
  GetContest: 1 << 0,
  ManageContest: 1 << 1,
  GetMonitor: 1 << 2,
  ListUsersSubmissions: 1 << 3,
  ListOwnSubmissions: 1 << 4,
  GetOwnSubmission: 1 << 5,
  GetOtherUserSubmission: 1 << 6,
  CreateSubmission: 1 << 7,
  GetSubmissionDetails: 1 << 8,
} as const;

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
    return this.user !== null;
  }

  isGlobalAdmin(): boolean {
    return this.user?.role === "admin";
  }

  // Contest permissions

  isContestOwner(contest: ContestModel): boolean {
    if (!this.user?.id) {
      return false;
    }
    if (contest.created_by && this.user.id === contest.created_by) {
      return true;
    }
    if (contest.owner?.id && this.user.id === contest.owner.id) {
      return true;
    }
    return false;
  }

  canViewContest(contest: ContestModel): boolean {
    if (this.isGlobalAdmin() || this.isContestOwner(contest)) {
      return true;
    }

    if (contest.visibility === "public") {
      return true;
    }

    if (!this.isAuthenticated()) {
      return false;
    }

    if (this.permissionsMask !== null) {
      return (this.permissionsMask & ContestPermissionMasks.GetContest) !== 0;
    }

    return this.contestRole !== null;
  }

  canViewProblems(contest: ContestModel): boolean {
    if (this.isGlobalAdmin() || this.isContestOwner(contest)) {
      return true;
    }
    if (this.canManageContest(contest)) {
      return true;
    }

    const hasStarted = !contest.start_time || new Date(contest.start_time) <= new Date();
    if (!hasStarted) {
      return false;
    }

    if (contest.visibility === "public") {
      return true;
    }

    if (this.permissionsMask !== null) {
      return (this.permissionsMask & ContestPermissionMasks.GetContest) !== 0;
    }

    return this.isAuthenticated() && this.contestRole !== null;
  }

  canSubmitSolution(contest: ContestModel): boolean {
    if (this.isGlobalAdmin() || this.isContestOwner(contest)) {
      return true;
    }
    if (this.canManageContest(contest)) {
      return true;
    }

    const hasStarted = !contest.start_time || new Date(contest.start_time) <= new Date();
    if (!hasStarted) {
      return false;
    }

    if (!this.isAuthenticated()) {
      return false;
    }

    if (this.permissionsMask !== null) {
      return (this.permissionsMask & ContestPermissionMasks.CreateSubmission) !== 0;
    }

    if (contest.visibility === "public") {
      return true;
    }

    return this.contestRole !== null;
  }

  canViewMySubmissions(contest: ContestModel): boolean {
    if (this.isGlobalAdmin() || this.isContestOwner(contest)) {
      return true;
    }
    if (this.canManageContest(contest)) {
      return true;
    }

    const hasStarted = !contest.start_time || new Date(contest.start_time) <= new Date();
    if (!hasStarted) {
      return false;
    }

    if (!this.isAuthenticated()) {
      return false;
    }

    if (this.permissionsMask !== null) {
      return (this.permissionsMask & ContestPermissionMasks.ListOwnSubmissions) !== 0;
    }

    if (contest.visibility === "public") {
      return true;
    }

    return this.contestRole !== null;
  }

  canViewAllSubmissions(contest: ContestModel): boolean {
    if (this.isGlobalAdmin() || this.isContestOwner(contest)) {
      return true;
    }

    if (this.permissionsMask !== null) {
      return (this.permissionsMask & ContestPermissionMasks.ListUsersSubmissions) !== 0;
    }

    if (!this.contestRole) {
      return false;
    }

    const requiredScope = (contest.submissions_list_scope ?? "moderator") as ContestScope;
    return hasRequiredRole(this.contestRole, requiredScope);
  }

  canViewSubmissionDetails(contest: ContestModel): boolean {
    if (this.isGlobalAdmin() || this.isContestOwner(contest)) {
      return true;
    }

    if (this.permissionsMask !== null) {
      return (this.permissionsMask & ContestPermissionMasks.GetSubmissionDetails) !== 0;
    }

    if (!this.contestRole) {
      return false;
    }

    const requiredScope = (contest.submission_details_scope ?? "moderator") as ContestScope;
    return hasRequiredRole(this.contestRole, requiredScope);
  }

  canViewMonitor(contest: ContestModel): boolean {
    if (this.isGlobalAdmin() || this.isContestOwner(contest)) {
      return true;
    }

    if (this.permissionsMask !== null) {
      return (this.permissionsMask & ContestPermissionMasks.GetMonitor) !== 0;
    }

    if (!this.contestRole) {
      return false;
    }

    const requiredScope = (contest.monitor_scope ?? "participant") as ContestScope;
    return hasRequiredRole(this.contestRole, requiredScope);
  }

  canManageContest(contest: ContestModel): boolean {
    if (this.isGlobalAdmin() || this.isContestOwner(contest)) {
      return true;
    }

    if (this.permissionsMask !== null) {
      return (this.permissionsMask & ContestPermissionMasks.ManageContest) !== 0;
    }

    if (!this.contestRole) {
      return false;
    }

    return this.contestRole === "owner" || this.contestRole === "moderator";
  }

  canDeleteContest(contest: ContestModel): boolean {
    if (this.isGlobalAdmin() || this.isContestOwner(contest)) {
      return true;
    }

    return this.contestRole === "owner";
  }

  canManageContestParticipants(contest: ContestModel): boolean {
    return this.canManageContest(contest);
  }

  canRejudgeSubmissions(contest: ContestModel): boolean {
    return this.canManageContest(contest);
  }

  // Problem permissions

  canViewProblem(problem: ProblemModel): boolean {
    if (!this.isAuthenticated()) {
      return false;
    }

    if (this.isGlobalAdmin() || (problem.created_by && this.user?.id === problem.created_by)) {
      return true;
    }

    return problem.visibility === "public" || (!problem.is_private && problem.visibility !== "private");
  }

  canEditProblem(problem: ProblemModel): boolean {
    if (!this.isAuthenticated()) {
      return false;
    }

    if (this.isGlobalAdmin() || (problem.created_by && this.user?.id === problem.created_by)) {
      return true;
    }

    return false;
  }

  canDeleteProblem(problem: ProblemModel): boolean {
    if (!this.isAuthenticated()) {
      return false;
    }

    if (this.isGlobalAdmin() || (problem.created_by && this.user?.id === problem.created_by)) {
      return true;
    }

    return false;
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
    if (this.isGlobalAdmin()) {
      return true;
    }
    if (!this.orgRole) {
      return false;
    }
    return ORG_ROLE_HIERARCHY[this.orgRole] >= ORG_ROLE_HIERARCHY['admin'];
  }

  canCreateTeam(): boolean {
    return this.canManageOrgMembers();
  }

  canDeleteOrg(): boolean {
    if (this.isGlobalAdmin()) {
      return true;
    }
    return this.orgRole === 'owner';
  }

  canManageTeamMembers(): boolean {
    return this.canManageOrgMembers();
  }
}

export const canManageOrgMembers = async (orgLogin: string): Promise<boolean> => {
  const [, me] = await api.getMe();
  const currentUser = me?.user ?? null;
  if (!currentUser) {
    return false;
  }
  if (currentUser.role === "admin") {
    return true;
  }

  let page = 1;
  while (page <= 10) {
    const [error, data] = await api.listOrganizationMembers({login: orgLogin, page, pageSize: 100});
    if (error || !data || !data.members) {
      return false;
    }

    const member = data.members.find((m) => m.user_id === currentUser.id);
    if (member) {
      return member.role === "owner" || member.role === "admin";
    }

    const total = data.pagination?.total ?? 0;
    const totalPages = Math.ceil(total / 100);
    if (page >= totalPages || totalPages === 0) {
      break;
    }
    page++;
  }

  return false;
};
