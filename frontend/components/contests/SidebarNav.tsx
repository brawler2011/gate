import { Box, Button, Stack } from "@mantine/core";
import Link from "next/link";
import React from "react";
import { getTabStyles } from "./get-tab-styles";

type NavSection = {
  key: string;
  label: string;
  icon: React.ComponentType<any>;
};

interface SidebarNavProps {
  contestId: string;
  activeSection: string;
  sections: readonly NavSection[];
}

export function SidebarNav({
  contestId,
  activeSection,
  sections,
}: SidebarNavProps) {
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
              href={`/contests/${contestId}/manage?section=${section.key}`}
              style={{ textDecoration: "none" }}
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
}
