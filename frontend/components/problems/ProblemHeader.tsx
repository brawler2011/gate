import {ActionIcon, Group, Stack, Title, Tooltip} from "@mantine/core";
import {IconEdit} from "@tabler/icons-react";
import React, {type ReactNode} from "react";

import {ProblemLimits} from "./ProblemLimits";

export interface ProblemHeaderProps {
  title: string;
  letter?: string;
  timeLimit: number;
  memoryLimit: number;
  isManager?: boolean;
  problemId?: string;
  orgLogin?: string;
}

export const ProblemHeader = ({
  title,
  letter = "A",
  timeLimit,
  memoryLimit,
  isManager,
  problemId,
  orgLogin,
}: ProblemHeaderProps): ReactNode => {
  return (
    <Stack align="center" gap={0} w="fit-content" mx="auto" mb="sm">
      <Group gap="xs" align="center" justify="center">
        <Title order={2}>
          {letter}. {title}
        </Title>
        {isManager && problemId && (
          <Tooltip label="Редактировать задачу" withArrow>
            <ActionIcon
              component="a"
              href={orgLogin ? `/${orgLogin}/problems/${problemId}` : `/problems/${problemId}`}
              target="_blank"
              rel="noopener noreferrer"
              variant="subtle"
              color="gray"
              size="md"
            >
              <IconEdit size={18} />
            </ActionIcon>
          </Tooltip>
        )}
      </Group>
      <ProblemLimits timeLimit={timeLimit} memoryLimit={memoryLimit} />
    </Stack>
  );
};
