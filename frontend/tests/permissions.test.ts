import {describe, it, expect} from "bun:test";

import {ContestModel, type ProblemModel, type UserModel} from "@/contracts/core/v1";
import {PermissionChecker} from "@/lib/permissions";

describe("PermissionChecker", () => {
  const globalAdmin: UserModel = {
    id: "user-admin",
    username: "admin",
    role: "admin",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };

  const regularUser: UserModel = {
    id: "user-regular",
    username: "regular",
    role: "user",
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
  };

  const createContest = (overrides: Partial<ContestModel> = {}): ContestModel => ({
    id: "contest-1",
    login: "contest-1",
    organization_id: "org-1",
    organization_login: "org-1",
    title: "Test Contest",
    description: "Contest description",
    visibility: "private",
    monitor_scope: "participant",
    submissions_list_scope: "moderator",
    submissions_review_scope: "moderator",
    submission_details_scope: "moderator",
    freeze_status: ContestModel.freeze_status.AUTO,
    freeze_duration_minutes: 60,
    created_by: "creator-id",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    start_time: new Date(Date.now() - 3600000).toISOString(), // started 1 hr ago
    end_time: new Date(Date.now() + 3600000).toISOString(),   // ends in 1 hr
    enable_drafts: true,
    enable_upsolving: true,
    enable_virtual_contests: false,
    participation_mode: ContestModel.participation_mode.OPEN,
    ...overrides,
  });

  const createProblem = (overrides: Partial<ProblemModel> = {}): ProblemModel => ({
    id: "prob-1",
    organization_id: "org-1",
    organization_login: "org-1",
    title: "Test Problem",
    visibility: "private",
    created_by: "creator-id",
    time_limit: 1000,
    memory_limit: 256,
    is_template: false,
    legend: "",
    input_format: "",
    output_format: "",
    notes: "",
    scoring: "",
    legend_html: "",
    input_format_html: "",
    output_format_html: "",
    notes_html: "",
    scoring_html: "",
    samples: [],
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  });

  describe("Role inheritance & scopes", () => {
    it("Global Admin has full management access on everything", () => {
      const checker = new PermissionChecker(globalAdmin);
      const contest = createContest({visibility: "private"});

      expect(checker.isGlobalAdmin()).toBe(true);
      expect(checker.canViewContest(contest)).toBe(true);
      expect(checker.canViewProblems(contest)).toBe(true);
      expect(checker.canSubmitSolution(contest)).toBe(true);
      expect(checker.canManageContest(contest)).toBe(true);
      expect(checker.canDeleteContest(contest)).toBe(true);
      expect(checker.canViewMonitor(contest)).toBe(true);
    });

    it("Org Owner / Org Admin inherits owner permissions on all org contests", () => {
      const ownerChecker = new PermissionChecker(regularUser, null, "owner");
      const adminChecker = new PermissionChecker(regularUser, null, "admin");
      const memberChecker = new PermissionChecker(regularUser, null, "member");
      const contest = createContest({visibility: "private"});

      expect(ownerChecker.canManageContest(contest)).toBe(true);
      expect(ownerChecker.canDeleteContest(contest)).toBe(true);
      expect(adminChecker.canManageContest(contest)).toBe(true);
      expect(adminChecker.canDeleteContest(contest)).toBe(true);

      // Regular member does not have manage/delete access by default
      expect(memberChecker.canManageContest(contest)).toBe(false);
      expect(memberChecker.canDeleteContest(contest)).toBe(false);
    });

    it("Contest Owner has full management access", () => {
      const checker = new PermissionChecker(regularUser, "owner");
      const contest = createContest({visibility: "private"});

      expect(checker.canManageContest(contest)).toBe(true);
      expect(checker.canDeleteContest(contest)).toBe(true);
    });

    it("Contest Moderator can manage but cannot delete contest", () => {
      const checker = new PermissionChecker(regularUser, "moderator");
      const contest = createContest({visibility: "private"});

      expect(checker.canManageContest(contest)).toBe(true);
      expect(checker.canDeleteContest(contest)).toBe(false);
    });
  });

  describe("Participation mode & Public/Private contests", () => {
    it("Public contest with participation_mode=open grants submission to authenticated user", () => {
      const checker = new PermissionChecker(regularUser, null);
      const contest = createContest({
        visibility: "public",
        participation_mode: ContestModel.participation_mode.OPEN,
      });

      expect(checker.canViewContest(contest)).toBe(true);
      expect(checker.canSubmitSolution(contest)).toBe(true);
    });

    it("Public contest with participation_mode=invite_only does NOT grant submission to uninvited user", () => {
      const checker = new PermissionChecker(regularUser, null);
      const contest = createContest({
        visibility: "public",
        participation_mode: ContestModel.participation_mode.INVITE_ONLY,
      });

      expect(checker.canViewContest(contest)).toBe(true);
      expect(checker.canSubmitSolution(contest)).toBe(false);
    });

    it("Unauthenticated guest cannot submit even in public open contest", () => {
      const checker = new PermissionChecker(null, null);
      const contest = createContest({
        visibility: "public",
        participation_mode: ContestModel.participation_mode.OPEN,
      });

      expect(checker.canViewContest(contest)).toBe(true);
      expect(checker.canSubmitSolution(contest)).toBe(false);
    });
  });

  describe("Timing & Upsolving ABAC", () => {
    it("Before contest start, participant cannot view problems or submit, but moderator can", () => {
      const futureContest = createContest({
        start_time: new Date(Date.now() + 3600000).toISOString(),
        end_time: new Date(Date.now() + 7200000).toISOString(),
      });

      const participantChecker = new PermissionChecker(regularUser, "participant");
      const modChecker = new PermissionChecker(regularUser, "moderator");

      expect(participantChecker.canViewProblems(futureContest)).toBe(false);
      expect(participantChecker.canSubmitSolution(futureContest)).toBe(false);

      expect(modChecker.canViewProblems(futureContest)).toBe(true);
      expect(modChecker.canSubmitSolution(futureContest)).toBe(true);
    });

    it("After contest end, upsolving enabled allows submissions for users who can view contest", () => {
      const finishedContest = createContest({
        visibility: "public",
        start_time: new Date(Date.now() - 7200000).toISOString(),
        end_time: new Date(Date.now() - 3600000).toISOString(),
        enable_upsolving: true,
      });

      const userChecker = new PermissionChecker(regularUser, null);
      const guestChecker = new PermissionChecker(null, null);

      expect(userChecker.canSubmitSolution(finishedContest)).toBe(true);
      expect(guestChecker.canSubmitSolution(finishedContest)).toBe(false);
    });

    it("After contest end, upsolving disabled blocks submissions for regular users", () => {
      const finishedContest = createContest({
        visibility: "public",
        start_time: new Date(Date.now() - 7200000).toISOString(),
        end_time: new Date(Date.now() - 3600000).toISOString(),
        enable_upsolving: false,
      });

      const participantChecker = new PermissionChecker(regularUser, "participant");
      const modChecker = new PermissionChecker(regularUser, "moderator");

      expect(participantChecker.canSubmitSolution(finishedContest)).toBe(false);
      expect(modChecker.canSubmitSolution(finishedContest)).toBe(true);
    });
  });

  describe("Problem permissions", () => {
    it("Org Owner and Org Admin can edit and delete org problems", () => {
      const ownerChecker = new PermissionChecker(regularUser, null, "owner");
      const adminChecker = new PermissionChecker(regularUser, null, "admin");
      const memberChecker = new PermissionChecker(regularUser, null, "member");
      const problem = createProblem({visibility: "private"});

      expect(ownerChecker.canEditProblem(problem)).toBe(true);
      expect(ownerChecker.canDeleteProblem(problem)).toBe(true);
      expect(adminChecker.canEditProblem(problem)).toBe(true);
      expect(adminChecker.canDeleteProblem(problem)).toBe(true);

      expect(memberChecker.canEditProblem(problem)).toBe(false);
      expect(memberChecker.canDeleteProblem(problem)).toBe(false);
    });

    it("Public problem is viewable by anyone", () => {
      const checker = new PermissionChecker(null);
      const problem = createProblem({visibility: "public"});

      expect(checker.canViewProblem(problem)).toBe(true);
    });
  });
});
