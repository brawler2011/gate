"use client";

import { Paper, Stack, Text } from "@mantine/core";
import type { ReactNode } from "react";

type Props = {
  title?: string;
  children: ReactNode;
};

export function SectionPaper({ title, children }: Props) {
  return (
    <Paper
      withBorder
      p="lg"
      radius="md"
      style={{ borderColor: "var(--mantine-color-default-border)" }}
    >
      <Stack gap="md">
        {title && (
          <Text fw={600} size="sm" tt="uppercase" c="dimmed" style={{ letterSpacing: "0.05em" }}>
            {title}
          </Text>
        )}
        {children}
      </Stack>
    </Paper>
  );
}
