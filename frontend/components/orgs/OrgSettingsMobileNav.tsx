"use client";

import {Box, Button, Group} from "@mantine/core";
import Link from "next/link";

import {
  getOrgSettingsIcon,
  getOrgSettingsNavTabStyles,
  type OrgSettingsNavSection,
} from "@/components/orgs/OrgSettingsNavShared";

import type {ReactNode} from "react";

interface OrgSettingsMobileNavProps {
  orgLogin: string;
  activeSection: string;
  sections: readonly OrgSettingsNavSection[];
}

export const OrgSettingsMobileNav = ({
  orgLogin,
  activeSection,
  sections,
}: OrgSettingsMobileNavProps): ReactNode => {
  return (
    <Box hiddenFrom="sm" style={{width: "100%"}}>
      <Group gap="xs" mb="md">
        {sections.map((section) => {
          const Icon = getOrgSettingsIcon(section.key);
          const isActive = activeSection === section.key;

          return (
            <Button
              key={section.key}
              component={Link}
              href={`/${orgLogin}/settings?section=${section.key}`}
              style={{textDecoration: "none"}}
              variant="transparent"
              size="xs"
              leftSection={<Icon size={16} color="currentColor" />}
              data-active={isActive || undefined}
              styles={getOrgSettingsNavTabStyles(isActive)}
            >
              {section.label}
            </Button>
          );
        })}
      </Group>
    </Box>
  );
};
