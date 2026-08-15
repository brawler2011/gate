"use client";

import {Box, Button, Stack} from "@mantine/core";
import Link from "next/link";

import {
  getOrgSettingsIcon,
  getOrgSettingsNavTabStyles,
  type OrgSettingsNavSection,
} from "@/components/orgs/OrgSettingsNavShared";

import type {ReactNode} from "react";

interface OrgSettingsSidebarNavProps {
  orgId: string;
  activeSection: string;
  sections: readonly OrgSettingsNavSection[];
}

export const OrgSettingsSidebarNav = ({
  orgId,
  activeSection,
  sections,
}: OrgSettingsSidebarNavProps): ReactNode => {
  return (
    <Box
      style={{
        width: 250,
        flexShrink: 0,
      }}
      visibleFrom="sm"
    >
      <Stack gap="xs">
        {sections.map((section) => {
          const Icon = getOrgSettingsIcon(section.key);
          const isActive = activeSection === section.key;

          return (
            <Button
              key={section.key}
              component={Link}
              href={`/orgs/${orgId}/settings?section=${section.key}`}
              style={{textDecoration: "none"}}
              variant="transparent"
              size="sm"
              leftSection={<Icon size={20} color="currentColor" />}
              fullWidth
              justify="flex-start"
              data-active={isActive || undefined}
              styles={getOrgSettingsNavTabStyles(isActive)}
            >
              {section.label}
            </Button>
          );
        })}
      </Stack>
    </Box>
  );
};
