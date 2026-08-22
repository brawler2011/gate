"use client";

import {Button, Select, Stack, TextInput, Textarea} from '@mantine/core';
import {useForm} from '@mantine/form';
import {notifications} from '@mantine/notifications';
import {useRouter} from 'next/navigation';
import {useState, type ReactNode} from 'react';

import {
  type UpdateOrganizationRequestModel,
  type OrganizationModel,
} from '@/contracts/core/v1';
import {api} from '@/lib/api';

type Props = { org: OrganizationModel };

const JOIN_POLICY_OPTIONS = [
  {label: "По заявкам (требуется одобрение)", value: "by_request"},
  {label: "Открытая (любой может вступить напрямую)", value: "open"},
  {label: "Только по приглашениям", value: "invite_only"},
];

export const OrgSettingsForm = ({org}: Props): ReactNode => {
  const router = useRouter();
  const [saving, setSaving] = useState(false);

  const form = useForm({
    initialValues: {
      name: org.name,
      login: org.login,
      description: org.description ?? '',
      join_policy: org.join_policy ?? 'by_request',
    },
    validate: {
      name: (v) => v.trim().length === 0 ? 'Название обязательно' : null,
      login: (v) => {
        const trimmed = v.trim();
        if (trimmed.length === 0) {
          return 'Логин обязателен';
        }
        if (trimmed.length < 3) {
          return 'Логин должен содержать не менее 3 символов';
        }
        if (trimmed.length > 32) {
          return 'Логин не должен превышать 32 символа';
        }
        if (trimmed.startsWith('@')) {
          return 'Логин не может начинаться с @';
        }
        if (!/^[a-z0-9][a-z0-9-]{1,30}[a-z0-9]$/.test(trimmed)) {
          return 'Логин может содержать только строчные латинские буквы, цифры и дефисы';
        }
        return null;
      },
    },
  });

  const handleSave = async (values: typeof form.values) => {
    setSaving(true);
    const trimmedLogin = values.login.trim();
    const [error] = await api.updateOrganization({
      login: org.login,
      requestBody: {
        name: values.name.trim(),
        login: trimmedLogin,
        description: values.description,
        join_policy: values.join_policy as UpdateOrganizationRequestModel.join_policy,
      },
    });
    setSaving(false);
    if (error) {
      notifications.show({title: 'Ошибка', message: error.message, color: 'red'});
      return;
    }
    notifications.show({title: 'Готово', message: 'Настройки обновлены', color: 'green'});
    form.resetDirty(values);
    if (trimmedLogin !== org.login) {
      router.push(`/${trimmedLogin}/settings`);
    } else {
      router.refresh();
    }
  };

  return (
    <form onSubmit={form.onSubmit(handleSave)}>
      <Stack gap="md">
        <TextInput label="Название" required {...form.getInputProps('name')} />
        <TextInput label="Логин (URL)" required {...form.getInputProps('login')} />
        <Select
          label="Политика вступления"
          description="Определяет, как новые участники могут присоединяться к организации"
          data={JOIN_POLICY_OPTIONS}
          {...form.getInputProps('join_policy')}
        />
        <Textarea label="Описание" autosize minRows={2} {...form.getInputProps('description')} />
        <Button type="submit" loading={saving} disabled={!form.isDirty()} w="fit-content">
          Сохранить
        </Button>
      </Stack>
    </form>
  );
};
