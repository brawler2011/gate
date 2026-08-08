import { PermissionChecker } from "@/lib/permissions";

import type { ContestModel, UserModel } from "@/contracts/core/v1";
import type { ContestRoleResponse } from "@/lib/contest-role";

export type ContestHeaderNavKey =
  | "tasks"
  | "submit"
  | "mysubmissions"
  | "allsubmissions"
  | "monitor"
  | "settings";

export type HeaderSecondaryNavIcon =
  | ContestHeaderNavKey
  | "contests"
  | "problems"
  | "teams"
  | "members"
  | "settings"
  | "users"
  | "blogs"
  | "orgs"
  | "submissions";

export type HeaderSecondaryNavItem = {
  key: string;
  label: string;
  href: string;
  icon?: HeaderSecondaryNavIcon;
  active?: boolean;
};

type BuildContestHeaderNavParams = {
  contest: ContestModel;
  user: UserModel | null;
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

  if (checker.canViewContest(contest)) {
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
      key: "settings",
      label: "Настройки",
      href: `/contests/${contest.id}/settings`,
      icon: "settings",
    });
  }

  return items.map((item) => ({
    ...item,
    active: item.key === activeTab,
  }));
}
