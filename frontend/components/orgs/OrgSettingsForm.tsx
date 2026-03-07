"use client";

import { updateOrganization } from '@/lib/actions';
import { Button, Stack, TextInput, Textarea } from '@mantine/core';
import { useForm } from '@mantine/form';
import { notifications } from '@mantine/notifications';
import { useRouter } from 'next/navigation';
import { useState } from 'react';
import type { OrganizationModel } from '@contracts/gateway/v1';

type Props = { org: OrganizationModel };

export function OrgSettingsForm({ org }: Props) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);

  const form = useForm({
    initialValues: { name: org.name, description: org.description ?? '' },
    validate: { name: (v) => v.trim().length === 0 ? 'Название обязательно' : null },
  });

  const handleSave = async (values: typeof form.values) => {
    setSaving(true);
    const [error] = await updateOrganization(org.id, values);
    setSaving(false);
    if (error) {
      notifications.show({ title: 'Ошибка', message: error.message, color: 'red' });
      return;
    }
    notifications.show({ title: 'Готово', message: 'Настройки обновлены', color: 'green' });
    form.resetDirty(values);
    router.refresh();
  };

  return (
    <form onSubmit={form.onSubmit(handleSave)}>
      <Stack gap="md">
        <TextInput label="Название" required {...form.getInputProps('name')} />
        <Textarea label="Описание" autosize minRows={2} {...form.getInputProps('description')} />
        <Button type="submit" loading={saving} disabled={!form.isDirty()} w="fit-content">
          Сохранить
        </Button>
      </Stack>
    </form>
  );
}
