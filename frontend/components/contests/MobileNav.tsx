import { Box, Button, Group } from "@mantine/core";
import Link from "next/link";
import React from "react";
import { getTabStyles } from "./get-tab-styles";

type NavSection = {
  key: string;
  label: string;
  icon: React.ComponentType<any>;
};

interface MobileNavProps {
  contestId: string;
  activeSection: string;
  sections: readonly NavSection[];
}

export function MobileNav({
  contestId,
  activeSection,
  sections,
}: MobileNavProps) {
  return (
    <Box hiddenFrom="sm" style={{ width: "100%" }}>
      <Group gap="xs" mb="md">
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
                size="xs"
                leftSection={<Icon size={16} color="currentColor" />}
                data-active={activeSection === section.key || undefined}
                styles={getTabStyles(activeSection === section.key)}
              >
                {section.label}
              </Button>
            </Link>
          );
        })}
      </Group>
    </Box>
  );
}
