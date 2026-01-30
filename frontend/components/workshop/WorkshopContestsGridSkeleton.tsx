import { Skeleton, Stack } from "@mantine/core";

export function WorkshopContestsGridSkeleton() {
  return (
    <Stack gap="xs">
      {Array.from({ length: 10 }).map((_, index) => (
        <Skeleton key={index} height={30} />
      ))}
    </Stack>
  );
}

