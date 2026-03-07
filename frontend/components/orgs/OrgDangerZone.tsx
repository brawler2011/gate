"use client";

import { deleteOrganization } from '@/lib/actions';
import { Alert, Button, Group, Modal, Stack, Text, TextInput } from '@mantine/core';
import { useDisclosure } from '@mantine/hooks';
import { notifications } from '@mantine/notifications';
import { useRouter } from 'next/navigation';
import { useState } from 'react';
import { IconAlertTriangle } from '@tabler/icons-react';

type Props = { orgId: string; orgName: string };

export function OrgDangerZone({ orgId, orgName }: Props) {
  const [opened, { open, close }] = useDisclosure(false);
  const [confirm, setConfirm] = useState('');
  const [loading, setLoading] = useState(false);
  const router = useRouter();

  const handleDelete = async () => {
    setLoading(true);
    const [error] = await deleteOrganization(orgId);
    setLoading(false);
    if (error) {
      notifications.show({ title: 'Ошибка', message: error.message, color: 'red' });
      return;
    }
    router.push('/orgs');
  };

  return (
    <>
      <Alert color="red" title="Опасная зона" icon={<IconAlertTriangle size={16} />}>
        <Stack gap="sm">
          <Text size="sm">Удаление организации необратимо. Все данные будут потеряны.</Text>
          <Button color="red" variant="outline" onClick={open} w="fit-content">
            Удалить организацию
          </Button>
        </Stack>
      </Alert>

      <Modal opened={opened} onClose={close} title="Подтвердите удаление" centered>
        <Stack>
          <Text size="sm">
            Введите <strong>{orgName}</strong> для подтверждения:
          </Text>
          <TextInput
            placeholder={orgName}
            value={confirm}
            onChange={(e) => setConfirm(e.currentTarget.value)}
          />
          <Group justify="flex-end">
            <Button variant="default" onClick={close}>Отмена</Button>
            <Button
              color="red"
              loading={loading}
              disabled={confirm !== orgName}
              onClick={handleDelete}
            >
              Удалить
            </Button>
          </Group>
        </Stack>
      </Modal>
    </>
  );
}
