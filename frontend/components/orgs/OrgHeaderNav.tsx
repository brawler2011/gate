"use client";

import {
  IconPuzzle,
  IconSettings,
  IconTrophy,
  IconUsers,
  IconUsersGroup,
} from "@tabler/icons-react";
import {usePathname} from "next/navigation";
import {type ReactNode} from "react";

import {AdaptiveTabs, type AdaptiveTabItem} from "@/components/shared/AdaptiveTabs";

export type OrgHeaderNavProps = {
  orgId: string;
  showMembersTab?: boolean;
};

export const OrgHeaderNav = ({
  orgId,
  showMembersTab = false,
}: OrgHeaderNavProps): ReactNode => {
  const pathname = usePathname();

  const getActiveTabKey = (): string => {
    if (!pathname) {
      return "contests";
    }
    if (pathname.includes(`/orgs/${orgId}/problems`)) {
      return "problems";
    }
    if (pathname.includes(`/orgs/${orgId}/teams`)) {
      return "teams";
    }
    if (pathname.includes(`/orgs/${orgId}/members`)) {
      return "members";
    }
    if (pathname.includes(`/orgs/${orgId}/settings`)) {
      return "settings";
    }
    return "contests";
  };

  const activeTab = getActiveTabKey();

  const items: AdaptiveTabItem[] = [
    {
      key: "contests",
      label: "Контесты",
      href: `/orgs/${orgId}`,
      icon: IconTrophy,
      active: activeTab === "contests",
    },
    {
      key: "problems",
      label: "Задачи",
      href: `/orgs/${orgId}/problems`,
      icon: IconPuzzle,
      active: activeTab === "problems",
    },
    {
      key: "teams",
      label: "Команды",
      href: `/orgs/${orgId}/teams`,
      icon: IconUsersGroup,
      active: activeTab === "teams",
    },
  ];

  if (showMembersTab) {
    items.push({
      key: "members",
      label: "Участники",
      href: `/orgs/${orgId}/members`,
      icon: IconUsers,
      active: activeTab === "members",
    });
  }

  items.push({
    key: "settings",
    label: "Настройки",
    href: `/orgs/${orgId}/settings`,
    icon: IconSettings,
    active: activeTab === "settings",
  });

  return <AdaptiveTabs items={items} />;
};
