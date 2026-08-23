import {Stack, Text} from "@mantine/core";
import React, {type ReactNode} from "react";

const prettifyTimeLimit = (timeLimit: number): string => {
  if (timeLimit % 1000 === 0) {
    return `${timeLimit / 1000} сек`;
  }
  return `${timeLimit} мс`;
};

const prettifyMemoryLimit = (memoryLimit: number): string => {
  if (memoryLimit % 1000 === 0) {
    return `${memoryLimit / 1000} ГБ`;
  }
  return `${memoryLimit} МБ`;
};

export interface ProblemLimitsProps {
  timeLimit: number;
  memoryLimit: number;
}

export const ProblemLimits = ({timeLimit, memoryLimit}: ProblemLimitsProps): ReactNode => {
  return (
    <Stack align="center" gap={0}>
      <Text>
        ограничение по времени: {prettifyTimeLimit(timeLimit)}
      </Text>
      <Text>
        ограничение по памяти: {prettifyMemoryLimit(memoryLimit)}
      </Text>
    </Stack>
  );
};
