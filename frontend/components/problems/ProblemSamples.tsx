import {Flex, Paper, Stack, Text, Title} from "@mantine/core";
import React, {type ReactNode} from "react";

import {ProblemCopyableBlock} from "./ProblemCopyableBlock";

interface ProblemSampleItem {
  input: string;
  output: string;
}

export interface ProblemSamplesProps {
  samples: ProblemSampleItem[];
  title?: string;
}

export const ProblemSamples = ({
  samples,
  title = "Примеры",
}: ProblemSamplesProps): ReactNode => {
  if (!samples || samples.length === 0) {
    return null;
  }

  return (
    <Stack gap="xs">
      <Title order={3}>{title}</Title>
      <Stack gap="md">
        {samples.map((sample, index) => (
          <Paper
            key={index}
            withBorder
            p="md"
            radius="md"
            bg="light-dark(var(--mantine-color-white), var(--mantine-color-dark-7))"
          >
            <Stack gap="xs">
              <Text fw={700} size="sm" c="dimmed">
                Пример {index + 1}
              </Text>
              <Flex gap="md" wrap="wrap">
                <ProblemCopyableBlock label="Входные данные" value={sample.input} />
                <ProblemCopyableBlock label="Выходные данные" value={sample.output} />
              </Flex>
            </Stack>
          </Paper>
        ))}
      </Stack>
    </Stack>
  );
};
