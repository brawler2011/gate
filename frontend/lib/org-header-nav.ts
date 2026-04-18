import type { HeaderSecondaryNavItem } from "@/lib/contest-header-nav";

export const ORG_OVERVIEW_TABS = [
  "contests",
  "problems",
  "teams",
  "members",
] as const;

export type OrgOverviewTab = (typeof ORG_OVERVIEW_TABS)[number];
export type OrgHeaderNavKey = OrgOverviewTab | "settings" | "workshop";

type BuildOrgHeaderNavParams = {
  orgId: string;
  activeTab: OrgHeaderNavKey;
};

function buildOrgOverviewHref(
  orgId: string,
  tab: OrgOverviewTab,
): string {
  if (tab === "contests") {
    return `/orgs/${orgId}`;
  }

  return `/orgs/${orgId}?tab=${tab}`;
}

export function buildOrgHeaderNav({
  orgId,
  activeTab,
}: BuildOrgHeaderNavParams): HeaderSecondaryNavItem[] {
  const items: HeaderSecondaryNavItem[] = [
    {
      key: "contests",
      label: "Контесты",
      href: buildOrgOverviewHref(orgId, "contests"),
      icon: "contests",
    },
    {
      key: "problems",
      label: "Задачи",
      href: buildOrgOverviewHref(orgId, "problems"),
      icon: "problems",
    },
    {
      key: "teams",
      label: "Команды",
      href: buildOrgOverviewHref(orgId, "teams"),
      icon: "teams",
    },
    {
      key: "members",
      label: "Участники",
      href: buildOrgOverviewHref(orgId, "members"),
      icon: "members",
    },
    {
      key: "settings",
      label: "Настройки",
      href: `/orgs/${orgId}/settings`,
      icon: "settings",
    },
    {
      key: "workshop",
      label: "Мастерская",
      href: `/workshop?org_id=${orgId}`,
      icon: "workshop",
    },
  ];

  return items.map((item) => ({
    ...item,
    active: item.key === activeTab,
  }));
}
