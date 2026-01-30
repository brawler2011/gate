import { Input, Stack } from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";
import { WorkshopContestsGridSkeleton } from "./WorkshopContestsGridSkeleton";

export function WorkshopContestsContentSkeleton() {
  return (
    <Stack gap="sm">
      <Input
        placeholder="Поиск контестов..."
        leftSection={<IconSearch size={16} />}
        disabled
        radius="md"
        size="md"
      />
      <WorkshopContestsGridSkeleton />
    </Stack>
  );
}

