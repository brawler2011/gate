"use client";

import {Skeleton, Stack, Tabs} from "@mantine/core";
import {IconTrophy} from "@tabler/icons-react";

export const UserContestsSkeleton = () => {
  return (
    <Tabs defaultValue="contests">
      <Tabs.List>
        <Tabs.Tab value="contests" leftSection={<IconTrophy size={16} />}>
          Контесты
        </Tabs.Tab>
      </Tabs.List>

      <Tabs.Panel value="contests" pt="md">
        <Stack gap="xs">
          <Skeleton height={40} radius="sm" />
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
          <Skeleton height={35} radius="sm" />
        </Stack>
      </Tabs.Panel>
    </Tabs>
  );
};
