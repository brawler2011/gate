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
  orgLogin: string;
  showMembersTab?: boolean;
};

export const OrgHeaderNav = ({
  orgLogin,
  showMembersTab = false,
}: OrgHeaderNavProps): ReactNode => {
  const pathname = usePathname();

  const getActiveTabKey = (): string => {
    if (!pathname) {
      return "contests";
    }
    if (pathname.includes(`/${orgLogin}/problems`)) {
      return "problems";
    }
    if (pathname.includes(`/${orgLogin}/teams`)) {
      return "teams";
    }
    if (pathname.includes(`/${orgLogin}/members`)) {
      return "members";
    }
    if (pathname.includes(`/${orgLogin}/settings`)) {
      return "settings";
    }
    return "contests";
  };

  const activeTab = getActiveTabKey();

  const items: AdaptiveTabItem[] = [
    {
      key: "contests",
      label: "Контесты",
      href: `/${orgLogin}`,
      icon: IconTrophy,
      active: activeTab === "contests",
    },
    {
      key: "problems",
      label: "Задачи",
      href: `/${orgLogin}/problems`,
      icon: IconPuzzle,
      active: activeTab === "problems",
    },
    {
      key: "teams",
      label: "Команды",
      href: `/${orgLogin}/teams`,
      icon: IconUsersGroup,
      active: activeTab === "teams",
    },
  ];

  if (showMembersTab) {
    items.push({
      key: "members",
      label: "Участники",
      href: `/${orgLogin}/members`,
      icon: IconUsers,
      active: activeTab === "members",
    });
  }

  items.push({
    key: "settings",
    label: "Настройки",
    href: `/${orgLogin}/settings`,
    icon: IconSettings,
    active: activeTab === "settings",
  });

  return <AdaptiveTabs items={items} />;
};
