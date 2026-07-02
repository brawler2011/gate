"use client";

import { updateContest } from "@/lib/actions";
import {
  Button,
  Stack,
  TextInput,
  Badge,
  Combobox,
  useCombobox,
  InputBase,
  Input,
  Text,
  Divider,
} from "@mantine/core";
import { useForm } from "@mantine/form";
import { notifications } from "@mantine/notifications";
import { useRouter } from "next/navigation";
import { useState } from "react";
import type * as corev1 from "@contracts/core/v1";
import { APP_COLORS } from "@/lib/theme/colors";
import { StatusMessage } from '@/components/shared/StatusMessage';

interface SettingsSectionProps {
  contest: corev1.ContestModel;
}

const SCOPE_OPTIONS = [
  { label: "Участник", value: "participant", color: "gray" },
  { label: "Модератор", value: "moderator", color: "yellow" },
  { label: "Создатель", value: "owner", color: "red" },
];

const VISIBILITY_OPTIONS = [
  { label: "Публичный", value: "public", color: "green" },
  { label: "Приватный", value: "private", color: "red" },
];

interface CustomSelectProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: typeof SCOPE_OPTIONS;
  description?: string;
}

function CustomSelect({ label, value, onChange, options, description }: CustomSelectProps) {
  const combobox = useCombobox({
    onDropdownClose: () => combobox.resetSelectedOption(),
  });

  const selected = options.find(item => item.value === value);

  return (
    <Input.Wrapper label={label} description={description}>
      <Combobox store={combobox} onOptionSubmit={(val) => { onChange(val); combobox.closeDropdown(); }}>
        <Combobox.Target>
          <InputBase
            component="button"
            type="button"
            pointer
            rightSection={<Combobox.Chevron />}
            onClick={() => combobox.toggleDropdown()}
            rightSectionPointerEvents="none"
          >
            {selected && <Badge color={selected.color} variant="filled" tt="none">{selected.label}</Badge>}
          </InputBase>
        </Combobox.Target>
        <Combobox.Dropdown>
          <Combobox.Options>
            {options.map((item) => (
              <Combobox.Option value={item.value} key={item.value}>
                <Badge color={item.color} variant="filled" tt="none">{item.label}</Badge>
              </Combobox.Option>
            ))}
          </Combobox.Options>
        </Combobox.Dropdown>
      </Combobox>
    </Input.Wrapper>
  );
}

function toLocalDatetimeString(dateStr: string | null | undefined): string {
  if (!dateStr) return "";
  const date = new Date(dateStr);
  if (isNaN(date.getTime())) return "";
  const pad = (n: number) => n.toString().padStart(2, '0');
  const y = date.getFullYear();
  const m = pad(date.getMonth() + 1);
  const d = pad(date.getDate());
  const h = pad(date.getHours());
  const min = pad(date.getMinutes());
  return `${y}-${m}-${d}T${h}:${min}`;
}

export function SettingsSection({ contest }: SettingsSectionProps) {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  const form = useForm({
    initialValues: {
      title: contest.title,
      description: contest.description,
      visibility: contest.visibility,
      monitor_scope: contest.monitor_scope,
      submissions_list_scope: contest.submissions_list_scope,
      submissions_review_scope: contest.submissions_review_scope,
      start_time: toLocalDatetimeString(contest.start_time),
      end_time: toLocalDatetimeString(contest.end_time),
    },
  });

  const handleSave = async (values: typeof form.values) => {
    setSaving(true);
    const payload = {
      ...values,
      start_time: values.start_time ? new Date(values.start_time).toISOString() : null,
      end_time: values.end_time ? new Date(values.end_time).toISOString() : null,
    };
    const [error] = await updateContest(contest.id, payload);
    setSaving(false);

    if (error) {
      console.error("Failed to update contest:", error);
      notifications.show({
        title: "Ошибка",
        message: error.message || "Не удалось обновить настройки",
        color: "red",
      });
      setStatusMessage({
        type: "error",
        message: "Не удалось обновить настройки",
      });
      return;
    }

    setStatusMessage({
      type: "success",
      message: "Настройки контеста обновлены",
    });
    
    router.refresh();
  };

  return (
    <>
      <form onSubmit={form.onSubmit(handleSave)}>
        <Stack gap="md">
          <TextInput
            label="Название"
            placeholder="Введите название контеста"
            required
            {...form.getInputProps("title")}
          />

          <TextInput
            label="Описание"
            placeholder="Введите описание контеста"
            {...form.getInputProps("description")}
          />

          <TextInput
            label="Время начала контеста"
            type="datetime-local"
            {...form.getInputProps("start_time")}
          />

          <TextInput
            label="Время окончания контеста"
            type="datetime-local"
            {...form.getInputProps("end_time")}
          />

          <CustomSelect
            label="Видимость"
            value={form.values.visibility}
            onChange={(value) => form.setFieldValue('visibility', value)}
            options={VISIBILITY_OPTIONS}
          />

          <Divider my="sm" />
          
          <Text size="sm" c="dimmed" mb="md">
            Следующие настройки определяют минимальную роль пользователя для доступа к различным функциям контеста
          </Text>

          <CustomSelect
            label="Доступ к монитору"
            value={form.values.monitor_scope}
            onChange={(value) => form.setFieldValue('monitor_scope', value)}
            options={SCOPE_OPTIONS}
          />

          <CustomSelect
            label="Просмотр списка посылок"
            value={form.values.submissions_list_scope}
            onChange={(value) => form.setFieldValue('submissions_list_scope', value)}
            options={SCOPE_OPTIONS}
          />

          <CustomSelect
            label="Просмотр кода посылок"
            value={form.values.submissions_review_scope}
            onChange={(value) => form.setFieldValue('submissions_review_scope', value)}
            options={SCOPE_OPTIONS}
          />

          <Button type="submit" loading={saving} fullWidth color={APP_COLORS.contests}>
            Сохранить изменения
          </Button>
        </Stack>
      </form>

      <StatusMessage
        type={statusMessage?.type || "success"}
        message={statusMessage?.message || ""}
        opened={!!statusMessage}
        onClose={() => setStatusMessage(null)}
      />
    </>
  );
}
