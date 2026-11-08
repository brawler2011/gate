import {describe, expect, it} from "bun:test";

import {PermissionChecker, parseContestRoleResponse} from "@/lib/permissions";

import type {ContestModel, ProblemModel} from "@/contracts/core/v1/schema";

describe("server capability decisions", () => {
  const checker = new PermissionChecker();
  it("does not infer submit or deletion from identity, public visibility or upsolving", () => {
    const contest = {capabilities: ["view", "manage"], visibility: "public", enable_upsolving: true} as ContestModel;
    expect(checker.canViewContest(contest)).toBe(true);
    expect(checker.canManageContest(contest)).toBe(true);
    expect(checker.canDeleteContest(contest)).toBe(false);
    expect(checker.canSubmitSolution(contest)).toBe(false);
    expect(checker.canViewProblems(contest)).toBe(false);
  });
  it("honors server grants independently for source problems and contest operations", () => {
    expect(checker.canSubmitSolution({capabilities: ["submit"]} as ContestModel)).toBe(true);
    expect(checker.canEditProblem({capabilities: ["view"]} as ProblemModel)).toBe(false);
    expect(checker.canDeleteProblem({capabilities: ["edit"]} as ProblemModel)).toBe(false);
  });
  it("preserves capabilities even without a direct contest role", () => {
    expect(parseContestRoleResponse({role: "", capabilities: ["view"]})).toEqual({role: "", capabilities: ["view"]});
    expect(parseContestRoleResponse({role: "owner"})).toBeNull();
    expect(parseContestRoleResponse({role: "participant", capabilities: [1]})).toBeNull();
  });
});
