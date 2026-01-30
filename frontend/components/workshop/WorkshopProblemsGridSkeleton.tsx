import { Card, Group, SimpleGrid, Skeleton, Stack } from "@mantine/core";

export function WorkshopProblemsGridSkeleton() {
  return (
    <SimpleGrid
      cols={{ base: 1, xs: 2, sm: 2, md: 3, lg: 4, xl: 4 }}
      spacing={{ base: "xs", sm: "sm", md: "md" }}
    >
      {Array.from({ length: 20 }).map((_, index) => (
        <Card
          key={index}
          shadow="xs"
          p={{ base: "sm", sm: "md" }}
          radius="md"
          withBorder
          style={{ height: "100%" }}
        >
          <Stack gap="xs" h="100%" justify="space-between">
            {/* Title skeleton */}
            <Skeleton height={20} width="80%" />
            
            {/* Time/Memory stats skeleton */}
            <Group justify="space-between" gap="xs">
              <Skeleton height={14} width="60%" />
              <Skeleton height={14} width="40%" />
            </Group>
          </Stack>
        </Card>
      ))}
    </SimpleGrid>
  );
}

