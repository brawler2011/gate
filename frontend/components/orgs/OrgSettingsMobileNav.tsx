"use client";

import {
  getOrgSettingsIcon,
  getOrgSettingsNavTabStyles,
  type OrgSettingsNavSection,
} from "@/components/orgs/OrgSettingsNavShared";
import { Box, Button, Group } from "@mantine/core";
import Link from "next/link";

interface OrgSettingsMobileNavProps {
  orgId: string;
  activeSection: string;
  sections: readonly OrgSettingsNavSection[];
}

export function OrgSettingsMobileNav({
  orgId,
  activeSection,
  sections,
}: OrgSettingsMobileNavProps) {
  return (
    <Box hiddenFrom="sm" style={{ width: "100%" }}>
      <Group gap="xs" mb="md">
        {sections.map((section) => {
          const Icon = getOrgSettingsIcon(section.key);
          const isActive = activeSection === section.key;

          return (
            <Button
              key={section.key}
              component={Link}
              href={`/orgs/${orgId}/settings?section=${section.key}`}
              style={{ textDecoration: "none" }}
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
}
