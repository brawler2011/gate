"use client";
import type { OrganizationModel } from '@contracts/core/v1';
import { Card, Text, Title, Group } from '@mantine/core';
import { IconBuilding } from '@tabler/icons-react';
import Link from 'next/link';

type Props = { org: OrganizationModel };

export function OrgCard({ org }: Props) {
  return (
    <Card component={Link} href={`/orgs/${org.id}`} withBorder radius="md" padding="md">
      <Group mb="xs" gap="xs">
        <IconBuilding size={18} color="var(--mantine-color-teal-6)" />
        <Title order={5} lineClamp={1}>{org.name}</Title>
      </Group>
      {org.description && (
        <Text size="sm" c="dimmed" lineClamp={2}>{org.description}</Text>
      )}
      <Text size="xs" c="dimmed" mt="xs">
        {new Date(org.created_at).toLocaleDateString('ru-RU')}
      </Text>
    </Card>
  );
}
