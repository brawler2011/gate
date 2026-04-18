import type { SessionUser } from "@/lib/auth";
import type { ContestRoleResponse } from "@/lib/contest-role";
import { PermissionChecker } from "@/lib/permissions";
import type { ContestModel } from "@contracts/core/v1";

export type ContestHeaderNavKey =
  | "tasks"
  | "submit"
  | "mysubmissions"
  | "allsubmissions"
  | "monitor"
  | "manage";

export type HeaderSecondaryNavIcon =
  | ContestHeaderNavKey
  | "contests"
  | "problems"
  | "teams"
  | "members"
  | "settings";

export type HeaderSecondaryNavItem = {
  key: string;
  label: string;
  href: string;
  icon?: HeaderSecondaryNavIcon;
  active?: boolean;
};

type BuildContestHeaderNavParams = {
  contest: ContestModel;
  user: SessionUser;
  contestRole: ContestRoleResponse;
  activeTab: ContestHeaderNavKey;
};

export function buildContestHeaderNav({
  contest,
  user,
  contestRole,
  activeTab,
}: BuildContestHeaderNavParams): HeaderSecondaryNavItem[] {
  const checker = new PermissionChecker(user, contestRole?.role ?? null);

  const items: HeaderSecondaryNavItem[] = [];

  if (checker.canViewProblems(contest)) {
    items.push({
      key: "tasks",
      label: "Задачи",
      href: `/contests/${contest.id}`,
      icon: "tasks",
    });
  }

  if (checker.canSubmitSolution(contest)) {
    items.push({
      key: "submit",
      label: "Послать решение",
      href: `/contests/${contest.id}/submit`,
      icon: "submit",
    });
  }

  if (checker.canViewMySubmissions(contest) && user?.id) {
    items.push({
      key: "mysubmissions",
      label: "Мои посылки",
      href: `/contests/${contest.id}/mysubmissions?order=desc&userId=${user.id}`,
      icon: "mysubmissions",
    });
  }

  if (checker.canViewAllSubmissions(contest)) {
    items.push({
      key: "allsubmissions",
      label: "Все посылки",
      href: `/contests/${contest.id}/submissions?order=desc`,
      icon: "allsubmissions",
    });
  }

  if (checker.canViewMonitor(contest)) {
    items.push({
      key: "monitor",
      label: "Монитор",
      href: `/contests/${contest.id}/monitor`,
      icon: "monitor",
    });
  }

  if (checker.canManageContest(contest)) {
    items.push({
      key: "manage",
      label: "Управление",
      href: `/contests/${contest.id}/manage`,
      icon: "manage",
    });
  }

  return items.map((item) => ({
    ...item,
    active: item.key === activeTab,
  }));
}
