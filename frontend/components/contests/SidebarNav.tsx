import {Box, Button, Stack} from "@mantine/core";
import Link from "next/link";
import React from "react";

import {getTabStyles} from "./get-tab-styles";

import type {ReactNode} from "react";

type NavSection = {
  key: string;
  label: string;
  icon: React.ComponentType<{ size?: string | number; color?: string }>;
};

interface SidebarNavProps {
  contestId: string;
  activeSection: string;
  sections: readonly NavSection[];
}

export const SidebarNav = ({
  contestId,
  activeSection,
  sections,
}: SidebarNavProps): ReactNode => {
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
          const Icon = section.icon;
          return (
            <Link
              key={section.key}
              href={`/contests/${contestId}/settings?section=${section.key}`}
              style={{textDecoration: "none"}}
            >
              <Button
                variant="transparent"
                size="sm"
                leftSection={<Icon size={20} color="currentColor" />}
                fullWidth
                justify="flex-start"
                data-active={activeSection === section.key || undefined}
                styles={getTabStyles(activeSection === section.key)}
              >
                {section.label}
              </Button>
            </Link>
          );
        })}
      </Stack>
    </Box>
  );
};
