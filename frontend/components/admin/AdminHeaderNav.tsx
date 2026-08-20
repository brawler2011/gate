"use client";

import {
  IconBuilding,
  IconFileText,
  IconLayoutDashboard,
  IconNews,
  IconPuzzle,
  IconTrophy,
  IconUsers,
} from "@tabler/icons-react";
import {usePathname} from "next/navigation";
import {type ReactNode} from "react";

import {AdaptiveTabs, type AdaptiveTabItem} from "@/components/shared/AdaptiveTabs";

const ADMIN_NAV_ITEMS: AdaptiveTabItem[] = [
  {
    key: "dashboard",
    label: "Обзор",
    href: "/admin",
    icon: IconLayoutDashboard,
  },
  {
    key: "users",
    label: "Пользователи",
    href: "/admin/users",
    icon: IconUsers,
  },
  {
    key: "contests",
    label: "Контесты",
    href: "/admin/contests",
    icon: IconTrophy,
  },
  {
    key: "blogs",
    label: "Блоги",
    href: "/admin/blogs",
    icon: IconNews,
  },
  {
    key: "orgs",
    label: "Организации",
    href: "/admin/orgs",
    icon: IconBuilding,
  },
  {
    key: "problems",
    label: "Задачи",
    href: "/admin/problems",
    icon: IconPuzzle,
  },
  {
    key: "submissions",
    label: "Посылки",
    href: "/admin/submissions",
    icon: IconFileText,
  },
];

export const AdminHeaderNav = (): ReactNode => {
  const pathname = usePathname();

  const getActiveKey = (): string => {
    if (pathname?.includes("/admin/contests")) {
      return "contests";
    }
    if (pathname?.includes("/admin/blogs")) {
      return "blogs";
    }
    if (pathname?.includes("/admin/orgs")) {
      return "orgs";
    }
    if (pathname?.includes("/admin/problems")) {
      return "problems";
    }
    if (pathname?.includes("/admin/submissions")) {
      return "submissions";
    }
    if (pathname?.includes("/admin/users")) {
      return "users";
    }
    return "dashboard";
  };

  const activeKey = getActiveKey();

  const items = ADMIN_NAV_ITEMS.map((item) => ({
    ...item,
    active: item.key === activeKey,
  }));

  return <AdaptiveTabs items={items} />;
};
