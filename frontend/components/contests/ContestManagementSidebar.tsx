import { CreateContestForm } from '@/components/contests/CreateContestForm';
import { Paper, Stack, Title } from "@mantine/core";

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

