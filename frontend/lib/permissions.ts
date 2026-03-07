import type { ContestModel, ProblemModel } from "@contracts/core/v1";
import type { ContestRole } from "./contest-role";
import type { SessionUser } from "./auth";

/**
 * Permission checker utilities for frontend
 * These are client-side checks based on available data
 * Backend always performs authoritative permission checks
 */

export type ContestScope = "owner" | "moderator" | "participant";

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
function hasRequiredRole(userRole: ContestRole, requiredScope: ContestScope): boolean {
  return ROLE_HIERARCHY[userRole] >= ROLE_HIERARCHY[requiredScope];
}

export class PermissionChecker {
  private user: SessionUser;
  private contestRole: ContestRole | null;
  private orgRole: OrgRole | null;

  constructor(
    user: SessionUser,
    contestRole: ContestRole | null = null,
    orgRole: OrgRole | null = null
  ) {
    this.user = user;
    this.contestRole = contestRole;
    this.orgRole = orgRole;
  }

  isAuthenticated(): boolean {
    return this.user !== null;
  }

  isGlobalAdmin(): boolean {
    return this.user?.role === "admin";
  }

  // Contest permissions

  canViewContest(contest: ContestModel): boolean {
    // Global admin can view any contest
    if (this.isGlobalAdmin()) {
      return true;
    }

    // Public contests can be viewed by anyone (including unauthenticated users)
    if (contest.visibility === "public") {
      return true;
    }

    // Private contests require authentication and membership
    if (!this.isAuthenticated()) {
      return false;
    }

    // For authenticated users, backend will enforce actual visibility rules
    return this.contestRole !== null;
  }

  canViewProblems(contest: ContestModel): boolean {
    // Global admin всегда может
    if (this.isGlobalAdmin()) {
      return true;
    }

    // Public contests can be viewed by anyone (including unauthenticated users)
    if (contest.visibility === "public") {
      return true;
    }

    // Private contests require authentication and membership
    // TODO: Когда появится problems_view_scope в backend:
    // if (!this.contestRole) return false;
    // return hasRequiredRole(this.contestRole, contest.problems_view_scope as ContestScope);

    // Временно: все authenticated могут просматривать задачи приватных контестов если они участники
    return this.isAuthenticated() && this.contestRole !== null;
  }

  canSubmitSolution(contest: ContestModel): boolean {
    // Global admin всегда может
    if (this.isGlobalAdmin()) {
      return true;
    }

    // Submissions always require authentication (even for public contests)
    // because submissions must be associated with a user
    if (!this.isAuthenticated()) {
      return false;
    }

    // Public contests allow submissions from any authenticated user
    if (contest.visibility === "public") {
      return true;
    }

    // Private contests require contest membership
    // TODO: Когда появится problems_view_scope в backend:
    // if (!this.contestRole) return false;
    // return hasRequiredRole(this.contestRole, contest.problems_view_scope as ContestScope);

    // Временно: все authenticated участники могут отправлять решения в приватных контестах
    return this.contestRole !== null;
  }

  canViewMySubmissions(contest: ContestModel): boolean {
    // Global admin всегда может
    if (this.isGlobalAdmin()) {
      return true;
    }

    // Viewing own submissions requires authentication
    if (!this.isAuthenticated()) {
      return false;
    }

    // Public contests allow viewing own submissions for any authenticated user
    if (contest.visibility === "public") {
      return true;
    }

    // Private contests require contest membership
    // TODO: Когда появится problems_view_scope в backend:
    // if (!this.contestRole) return false;
    // return hasRequiredRole(this.contestRole, contest.problems_view_scope as ContestScope);

    // Временно: все authenticated участники могут просматривать свои посылки
    return this.contestRole !== null;
  }

  canViewAllSubmissions(contest: ContestModel): boolean {
    // Global admin всегда может
    if (this.isGlobalAdmin()) {
      return true;
    }

    // Проверяем роль в контесте
    if (!this.contestRole) {
      return false;
    }

    // Проверяем соответствие роли требуемому scope
    return hasRequiredRole(this.contestRole, contest.submissions_list_scope as ContestScope);
  }

  canViewMonitor(contest: ContestModel): boolean {
    // Global admin всегда может
    if (this.isGlobalAdmin()) {
      return true;
    }

    // Проверяем роль в контесте
    if (!this.contestRole) {
      return false;
    }

    // Проверяем соответствие роли требуемому scope
    return hasRequiredRole(this.contestRole, contest.monitor_scope as ContestScope);
  }

  canManageContest(contest: ContestModel): boolean {
    // Global admin всегда может управлять
    if (this.isGlobalAdmin()) {
      return true;
    }

    // Проверяем роль в контесте - только owner или moderator
    if (!this.contestRole) {
      return false;
    }

    return this.contestRole === "owner" || this.contestRole === "moderator";
  }

  canDeleteContest(contest: ContestModel): boolean {
    // Global admin can delete
    if (this.isGlobalAdmin()) {
      return true;
    }

    // Only contest owner can delete
    return this.contestRole === "owner";
  }

  canManageContestParticipants(contest: ContestModel): boolean {
    // Same as manage for now
    return this.canManageContest(contest);
  }

  // Problem permissions

  canViewProblem(problem: ProblemModel): boolean {
    if (!this.isAuthenticated()) {
      return false;
    }

    // Global admin can view any problem
    if (this.isGlobalAdmin()) {
      return true;
    }

    // Public problems can be viewed by any authenticated user
    if (!problem.is_private) {
      return true;
    }

    // Private problems - only admin can view
    return this.isGlobalAdmin();
  }

  canEditProblem(problem: ProblemModel): boolean {
    if (!this.isAuthenticated()) {
      return false;
    }

    // Global admin can edit any problem
    if (this.isGlobalAdmin()) {
      return true;
    }

    // For now, assume backend will check if user is owner/moderator
    return false;
  }

  canDeleteProblem(problem: ProblemModel): boolean {
    if (!this.isAuthenticated()) {
      return false;
    }

    // Only admin can delete
    return this.isGlobalAdmin();
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

  canDeleteUser(userId: string): boolean {
    if (!this.isAuthenticated()) {
      return false;
    }

    // Only admin can delete users
    return this.isGlobalAdmin();
  }

  // Org permissions

  canManageOrgMembers(): boolean {
    if (this.isGlobalAdmin()) return true;
    if (!this.orgRole) return false;
    return ORG_ROLE_HIERARCHY[this.orgRole] >= ORG_ROLE_HIERARCHY['admin'];
  }

  canCreateTeam(): boolean {
    return this.canManageOrgMembers();
  }

  canDeleteOrg(): boolean {
    if (this.isGlobalAdmin()) return true;
    return this.orgRole === 'owner';
  }

  canManageTeamMembers(): boolean {
    return this.canManageOrgMembers();
  }
}

/**
 * Create permission checker instance from user data
 */
export function createPermissionChecker(
  user: SessionUser,
  contestRole: ContestRole | null = null,
  orgRole: OrgRole | null = null
): PermissionChecker {
  return new PermissionChecker(user, contestRole, orgRole);
}
