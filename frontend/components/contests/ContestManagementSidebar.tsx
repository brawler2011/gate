import { Paper, Stack, Title } from "@mantine/core";

import { CreateContestForm } from '@/components/contests/CreateContestForm';

export const ContestManagementSidebar = () => {
  return (
    <Paper
      shadow="sm"
      radius="md"
      p="md"
      withBorder
      bg="var(--mantine-color-gray-light)"
    >
      <Stack gap="md">
        <Title order={4} size="h5">
          ⚙️ Управление
        </Title>
        <CreateContestForm />
      </Stack>
    </Paper>
  );
};

