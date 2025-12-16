import { Box, Button, Group } from "@mantine/core";
import Link from "next/link";
import React from "react";

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

export function MobileNav({ contestId, activeSection, sections }: MobileNavProps) {
    return (
        <Box hiddenFrom="sm" style={{ width: "100%" }}>
            <Group gap="xs" mb="md">
                {sections.map((section) => {
                    const Icon = section.icon;
                    return (
                        <Button
                            key={section.key}
                            component={Link}
                            href={`/contests/${contestId}/manage?section=${section.key}`}
                            variant={activeSection === section.key ? "filled" : "light"}
                            size="xs"
                            leftSection={<Icon size={16} />}
                        >
                            {section.label}
                        </Button>
                    );
                })}
            </Group>
        </Box>
    );
}

