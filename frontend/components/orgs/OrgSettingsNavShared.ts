import type { ButtonProps } from "@mantine/core";
import {
    IconAlertTriangle,
    IconSettings,
    IconUsers,
} from "@tabler/icons-react";
import type React from "react";

const ORG_SETTINGS_NAV_CONFIG = {
  settings: { label: "Настройки", icon: IconSettings },
  members: { label: "Участники", icon: IconUsers },
  danger: { label: "Опасная зона", icon: IconAlertTriangle },
} as const;

export type OrgSettingsNavSectionKey = keyof typeof ORG_SETTINGS_NAV_CONFIG;

export type OrgSettingsNavSection = {
  key: OrgSettingsNavSectionKey;
  label: string;
};

type OrgSettingsIconComponent = React.ComponentType<{
  size?: string | number;
  color?: string;
}>;

export const ORG_SETTINGS_NAV_SECTIONS: readonly OrgSettingsNavSection[] = Object.entries(
  ORG_SETTINGS_NAV_CONFIG,
).map(([key, value]) => ({
  key: key as OrgSettingsNavSectionKey,
  label: value.label,
}));

export const getOrgSettingsIcon = (
  sectionKey: OrgSettingsNavSectionKey,
): OrgSettingsIconComponent => ORG_SETTINGS_NAV_CONFIG[sectionKey].icon;

export function getOrgSettingsNavTabStyles(isActive: boolean): ButtonProps["styles"] {
  return {
    root: {
      backgroundColor: isActive
        ? "light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-5))"
        : "transparent",
      color:
        "light-dark(var(--mantine-color-dark-7), var(--mantine-color-dark-0))",
      border: "1px solid transparent",
      transition: "background-color 120ms ease, color 120ms ease",
      "&:hover": {
        backgroundColor: isActive
          ? "light-dark(var(--mantine-color-gray-2), var(--mantine-color-dark-5))"
          : "transparent",
        color:
          "light-dark(var(--mantine-color-black), var(--mantine-color-white))",
      },
    },
    section: {
      color: "inherit",
    },
  };
}
