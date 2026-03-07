"use client";

import { Button, Group } from '@mantine/core';
import Link from 'next/link';
import { IconSettings, IconTools } from '@tabler/icons-react';

type Props = { orgId: string };

export function OrgPageActions({ orgId }: Props) {
  return (
    <Group gap="xs">
      <Button
        component={Link}
        href={`/workshop?org_id=${orgId}`}
        variant="default"
        size="sm"
        leftSection={<IconTools size={16} />}
      >
        Мастерская
      </Button>
      <Button
        component={Link}
        href={`/orgs/${orgId}/settings`}
        variant="default"
        size="sm"
        leftSection={<IconSettings size={16} />}
      >
        Настройки
      </Button>
    </Group>
  );
}
