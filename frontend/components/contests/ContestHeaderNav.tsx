"use client";

import {
  IconDeviceDesktop,
  IconMail,
  IconPuzzle,
  IconSend,
  IconSettings,
  IconUser,
} from "@tabler/icons-react";
import {usePathname} from "next/navigation";
import {type ReactNode} from "react";

import {AdaptiveTabs, type AdaptiveTabItem} from "@/components/shared/AdaptiveTabs";
import {useSession} from "@/contexts/SessionContext";
import {PermissionChecker, type ContestRoleResponse} from "@/lib/permissions";

import type {ContestModel} from "@/contracts/core/v1";

export type ContestHeaderNavProps = {
  contest: ContestModel;
  contestRole?: ContestRoleResponse | null;
  orgLogin?: string;
};

export const ContestHeaderNav = ({
  contest,
  contestRole,
  orgLogin,
}: ContestHeaderNavProps): ReactNode => {
  const pathname = usePathname();
  const {user} = useSession();

  const org = orgLogin || contest.organization_login;
  const contestBase = `/${org}/contests/${contest.login}`;

  const checker = new PermissionChecker(
    user,
    contestRole?.role ?? null,
    null,
    contestRole?.permissionsMask ?? null,
  );

  const getActiveTabKey = (): string => {
    if (!pathname) {
      return "tasks";
    }
    if (pathname.includes(`/contests/${contest.login}/submit`)) {
      return "submit";
    }
    if (pathname.includes(`/contests/${contest.login}/mysubmissions`)) {
      return "mysubmissions";
    }
    if (pathname.includes(`/contests/${contest.login}/submissions`)) {
      return "allsubmissions";
    }
    if (pathname.includes(`/contests/${contest.login}/monitor`)) {
      return "monitor";
    }
    if (pathname.includes(`/contests/${contest.login}/settings`)) {
      return "settings";
    }
    return "tasks";
  };

  const activeTab = getActiveTabKey();
  const items: AdaptiveTabItem[] = [];

  if (checker.canViewContest(contest)) {
    items.push({
      key: "tasks",
      label: "Задачи",
      href: contestBase,
      icon: IconPuzzle,
      active: activeTab === "tasks",
    });
  }

  if (checker.canSubmitSolution(contest)) {
    items.push({
      key: "submit",
      label: "Послать решение",
      href: `${contestBase}/submit`,
      icon: IconSend,
      active: activeTab === "submit",
    });
  }

  if (checker.canViewMySubmissions(contest) && user?.id) {
    items.push({
      key: "mysubmissions",
      label: "Мои посылки",
      href: `${contestBase}/mysubmissions?order=desc&userId=${user.id}`,
      icon: IconUser,
      active: activeTab === "mysubmissions",
    });
  }

  if (checker.canViewAllSubmissions(contest)) {
    items.push({
      key: "allsubmissions",
      label: "Все посылки",
      href: `${contestBase}/submissions?order=desc`,
      icon: IconMail,
      active: activeTab === "allsubmissions",
    });
  }

  if (checker.canViewMonitor(contest)) {
    items.push({
      key: "monitor",
      label: "Монитор",
      href: `${contestBase}/monitor`,
      icon: IconDeviceDesktop,
      active: activeTab === "monitor",
    });
  }

  if (checker.canManageContest(contest)) {
    items.push({
      key: "settings",
      label: "Настройки",
      href: `${contestBase}/settings`,
      icon: IconSettings,
      active: activeTab === "settings",
    });
  }

  return <AdaptiveTabs items={items} />;
};
