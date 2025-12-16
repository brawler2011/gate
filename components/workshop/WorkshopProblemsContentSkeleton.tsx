import { Stack, TextInput } from "@mantine/core";
import { IconSearch } from "@tabler/icons-react";
import { WorkshopProblemsGridSkeleton } from "./WorkshopProblemsGridSkeleton";

export function WorkshopProblemsContentSkeleton() {
  return (
    <Stack gap="sm">
      <TextInput
        placeholder="Поиск задач..."
        leftSection={<IconSearch size={16} />}
        value=""
        disabled
        radius="md"
        size="md"
      />
      <WorkshopProblemsGridSkeleton />
    </Stack>
  );
}

