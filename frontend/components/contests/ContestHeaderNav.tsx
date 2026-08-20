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
import {PermissionChecker} from "@/lib/permissions";

import type {ContestModel} from "@/contracts/core/v1";
import type {ContestRoleResponse} from "@/lib/contest-role";

export type ContestHeaderNavProps = {
  contest: ContestModel;
  contestRole?: ContestRoleResponse | null;
};

export const ContestHeaderNav = ({
  contest,
  contestRole,
}: ContestHeaderNavProps): ReactNode => {
  const pathname = usePathname();
  const {user} = useSession();

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
    if (pathname.includes(`/contests/${contest.id}/submit`)) {
      return "submit";
    }
    if (pathname.includes(`/contests/${contest.id}/mysubmissions`)) {
      return "mysubmissions";
    }
    if (pathname.includes(`/contests/${contest.id}/submissions`)) {
      return "allsubmissions";
    }
    if (pathname.includes(`/contests/${contest.id}/monitor`)) {
      return "monitor";
    }
    if (pathname.includes(`/contests/${contest.id}/settings`)) {
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
      href: `/contests/${contest.id}`,
      icon: IconPuzzle,
      active: activeTab === "tasks",
    });
  }

  if (checker.canSubmitSolution(contest)) {
    items.push({
      key: "submit",
      label: "Послать решение",
      href: `/contests/${contest.id}/submit`,
      icon: IconSend,
      active: activeTab === "submit",
    });
  }

  if (checker.canViewMySubmissions(contest) && user?.id) {
    items.push({
      key: "mysubmissions",
      label: "Мои посылки",
      href: `/contests/${contest.id}/mysubmissions?order=desc&userId=${user.id}`,
      icon: IconUser,
      active: activeTab === "mysubmissions",
    });
  }

  if (checker.canViewAllSubmissions(contest)) {
    items.push({
      key: "allsubmissions",
      label: "Все посылки",
      href: `/contests/${contest.id}/submissions?order=desc`,
      icon: IconMail,
      active: activeTab === "allsubmissions",
    });
  }

  if (checker.canViewMonitor(contest)) {
    items.push({
      key: "monitor",
      label: "Монитор",
      href: `/contests/${contest.id}/monitor`,
      icon: IconDeviceDesktop,
      active: activeTab === "monitor",
    });
  }

  if (checker.canManageContest(contest)) {
    items.push({
      key: "settings",
      label: "Настройки",
      href: `/contests/${contest.id}/settings`,
      icon: IconSettings,
      active: activeTab === "settings",
    });
  }

  return <AdaptiveTabs items={items} />;
};
