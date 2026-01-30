import { Input, Stack } from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";
import { ContestsGridSkeleton } from "./ContestsGridSkeleton";

export function ContestsContentSkeleton() {
  return (
    <Stack gap="sm">
      <Input
        placeholder="Поиск контестов..."
        leftSection={<IconSearch size={16} />}
        disabled
        radius="md"
        size="md"
      />
      <ContestsGridSkeleton />
    </Stack>
  );
}

