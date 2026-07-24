import type { HeaderSecondaryNavItem } from "@/lib/contest-header-nav";

export type AdminHeaderNavKey =
  | "users"
  | "contests"
  | "blogs"
  | "orgs"
  | "problems"
  | "submissions";

export function buildAdminHeaderNav(pathname: string): HeaderSecondaryNavItem[] {
  const getActiveTab = (path: string): AdminHeaderNavKey => {
    if (path.includes("/admin/contests")) return "contests";
    if (path.includes("/admin/blogs")) return "blogs";
    if (path.includes("/admin/orgs")) return "orgs";
    if (path.includes("/admin/problems")) return "problems";
    if (path.includes("/admin/submissions")) return "submissions";
    return "users";
  };

  const activeTab = getActiveTab(pathname);

  return [
    {
      key: "users",
      label: "Пользователи",
      href: "/admin/users",
      icon: "users",
      active: activeTab === "users",
    },
    {
      key: "contests",
      label: "Контесты",
      href: "/admin/contests",
      icon: "contests",
      active: activeTab === "contests",
    },
    {
      key: "blogs",
      label: "Блоги",
      href: "/admin/blogs",
      icon: "blogs",
      active: activeTab === "blogs",
    },
    {
      key: "orgs",
      label: "Организации",
      href: "/admin/orgs",
      icon: "orgs",
      active: activeTab === "orgs",
    },
    {
      key: "problems",
      label: "Задачи",
      href: "/admin/problems",
      icon: "problems",
      active: activeTab === "problems",
    },
    {
      key: "submissions",
      label: "Посылки",
      href: "/admin/submissions",
      icon: "submissions",
      active: activeTab === "submissions",
    },
  ];
}
