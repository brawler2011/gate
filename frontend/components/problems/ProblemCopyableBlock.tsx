"use client";

import {ActionIcon, Box, Group, Stack, Text, Tooltip} from "@mantine/core";
import {useClipboard} from "@mantine/hooks";
import {IconCheck, IconCopy} from "@tabler/icons-react";
import React, {type ReactNode} from "react";

export interface ProblemCopyableBlockProps {
  label: string;
  value: string;
}

export const ProblemCopyableBlock = ({label, value}: ProblemCopyableBlockProps): ReactNode => {
  const clipboard = useClipboard({timeout: 2000});

  return (
    <Stack gap="xs" style={{flex: "1 1 200px", minWidth: 0}}>
      <Group justify="space-between" align="center" h={28}>
        <Text fw={600} size="sm">
          {label}
        </Text>
        <Tooltip label={clipboard.copied ? "Скопировано!" : "Скопировать"} position="top" withArrow>
          <ActionIcon
            variant="subtle"
            color={clipboard.copied ? "green" : "gray"}
            onClick={() => clipboard.copy(value)}
            size="sm"
          >
            {clipboard.copied ? <IconCheck size={16} /> : <IconCopy size={16} />}
          </ActionIcon>
        </Tooltip>
      </Group>
      <Box
        component="pre"
        p="xs"
        bg="light-dark(var(--mantine-color-gray-0), var(--mantine-color-dark-6))"
        style={{
          border: "1px solid light-dark(var(--mantine-color-gray-3), var(--mantine-color-dark-5))",
          borderRadius: "var(--mantine-radius-sm)",
          overflowX: "auto",
          fontFamily: "var(--mantine-font-monospace)",
          fontSize: "var(--mantine-font-size-sm)",
          whiteSpace: "pre",
          margin: 0,
        }}
      >
        {value}
      </Box>
    </Stack>
  );
};
