import {Box, Button, Center, Group, Loader, Stack, Text} from "@mantine/core";

import {useProblemDraft} from "./useDraft";

import type {ProblemDraft} from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

type Props = {
  problemId: string;
  children: (draft: ProblemDraft) => ReactNode;
};

/** Renders a workshop tab once the draft is loaded. */
export const DraftGate = ({problemId, children}: Props): ReactNode => {
  const {data, isPending, error, refetch} = useProblemDraft(problemId);
  if (isPending) {
    return (
      <Center p="xl">
        <Loader size="sm" />
      </Center>
    );
  }
  if (error || !data) {
    return (
      <Box p="lg">
        <Stack gap="sm" align="center">
          <Text c="red">{error?.message ?? "Не удалось загрузить черновик"}</Text>
          <Group>
            <Button variant="light" onClick={() => void refetch()}>
              Повторить
            </Button>
          </Group>
        </Stack>
      </Box>
    );
  }
  return <>{children(data)}</>;
};
