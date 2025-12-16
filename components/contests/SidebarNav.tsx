import { Box, Button, Stack } from "@mantine/core";
import Link from "next/link";
import React from "react";

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

export function SidebarNav({ contestId, activeSection, sections }: SidebarNavProps) {
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
                        <Button
                            key={section.key}
                            component={Link}
                            href={`/contests/${contestId}/manage?section=${section.key}`}
                            variant={activeSection === section.key ? "filled" : "light"}
                            size="sm"
                            leftSection={<Icon size={20} />}
                            fullWidth
                            justify="flex-start"
                        >
                            {section.label}
                        </Button>
                    );
                })}
            </Stack>
        </Box>
    );
}

