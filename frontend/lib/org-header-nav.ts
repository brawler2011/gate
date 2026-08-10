import type {HeaderSecondaryNavItem} from "@/lib/contest-header-nav";

type OrgOverviewTab = "contests" | "problems" | "teams" | "members";
type OrgHeaderNavKey = OrgOverviewTab | "settings";

type BuildOrgHeaderNavParams = {
  orgId: string;
  activeTab: OrgHeaderNavKey;
  showMembersTab?: boolean;
};

const buildOrgOverviewHref = (
  orgId: string,
  tab: OrgOverviewTab,
): string => {
  if (tab === "contests") {
    return `/orgs/${orgId}`;
  }

  return `/orgs/${orgId}/${tab}`;
};

export const buildOrgHeaderNav = ({
  orgId,
  activeTab,
  showMembersTab = false,
}: BuildOrgHeaderNavParams): HeaderSecondaryNavItem[] => {
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
  ];

  if (showMembersTab) {
    items.push({
      key: "members",
      label: "Участники",
      href: buildOrgOverviewHref(orgId, "members"),
      icon: "members",
    });
  }

  items.push({
    key: "settings",
    label: "Настройки",
    href: `/orgs/${orgId}/settings`,
    icon: "settings",
  });

  return items.map((item) => ({
    ...item,
    active: item.key === activeTab,
  }));
};
