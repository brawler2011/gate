import { Container, Group, Skeleton, Stack } from "@mantine/core";
import { UsersRoleFilter } from '@/components/users/UsersRoleFilter';
import { UsersSearchInput } from '@/components/users/UsersSearchInput';

export function UsersContentSkeleton() {
  return (
    <Container size="xl" py="xl">
      <Stack gap="lg">
        <Group grow>
          <UsersSearchInput />
          <UsersRoleFilter />
        </Group>

        <Stack gap="sm">
          {Array.from({ length: 10 }).map((_, index) => (
            <Skeleton key={index} height={35} radius="sm" />
          ))}
        </Stack>
      </Stack>
    </Container>
  );
}

