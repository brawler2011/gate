"use client";

import {
  Button,
  Stack,
  TextInput,
  NumberInput,
  Badge,
  Combobox,
  useCombobox,
  InputBase,
  Input,
  Text,
  Divider,
} from "@mantine/core";
import {useForm} from "@mantine/form";
import {notifications} from "@mantine/notifications";
import {useRouter} from "next/navigation";
import {useState} from "react";

import {StatusMessage} from '@/components/shared/StatusMessage';
import {api} from "@/lib/api";
import {APP_COLORS} from "@/lib/theme/colors";

import type * as corev1 from "@/contracts/core/v1";
import type {ReactNode} from "react";

interface SettingsSectionProps {
  contest: corev1.ContestModel;
}

const SCOPE_OPTIONS = [
  {label: "Участник", value: "participant", color: "gray"},
  {label: "Модератор", value: "moderator", color: "yellow"},
  {label: "Создатель", value: "owner", color: "red"},
];

const VISIBILITY_OPTIONS = [
  {label: "Публичный", value: "public", color: "green"},
  {label: "Приватный", value: "private", color: "red"},
];

const FREEZE_STATUS_OPTIONS = [
  {label: "Автоматически по таймеру", value: "auto", color: "blue"},
  {label: "Заморожен", value: "frozen", color: "orange"},
  {label: "Разморожен", value: "unfrozen", color: "green"},
];

interface CustomSelectProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: typeof SCOPE_OPTIONS;
  description?: string;
}

const CustomSelect = ({label, value, onChange, options, description}: CustomSelectProps) => {
  const combobox = useCombobox({
    onDropdownClose: () => combobox.resetSelectedOption(),
  });

  const selected = options.find(item => item.value === value);

  return (
    <Input.Wrapper label={label} description={description}>
      <Combobox store={combobox} onOptionSubmit={(val) => {
        onChange(val); combobox.closeDropdown(); 
      }}>
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
};

const toLocalDatetimeString = (dateStr: string | null | undefined): string => {
  if (!dateStr) {
    return "";
  }
  const date = new Date(dateStr);
  if (isNaN(date.getTime())) {
    return "";
  }
  const pad = (n: number) => n.toString().padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(
    date.getHours()
  )}:${pad(date.getMinutes())}`;
};

export const SettingsSection = ({contest}: SettingsSectionProps): ReactNode => {
  const router = useRouter();
  const [saving, setSaving] = useState(false);
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  const form = useForm<{
    login: string;
    title: string;
    description: string;
    visibility: string;
    monitor_scope: string;
    submissions_list_scope: string;
    submissions_review_scope: string;
    start_time: string;
    end_time: string;
    freeze_duration_minutes: number | string | undefined | null;
    freeze_status: corev1.UpdateContestRequestModel.freeze_status;
  }>({
    initialValues: {
      login: contest.login || "",
      title: contest.title,
      description: contest.description,
      visibility: contest.visibility,
      monitor_scope: contest.monitor_scope,
      submissions_list_scope: contest.submissions_list_scope,
      submissions_review_scope: contest.submissions_review_scope,
      start_time: toLocalDatetimeString(contest.start_time),
      end_time: toLocalDatetimeString(contest.end_time),
      freeze_duration_minutes: contest.freeze_duration_minutes ?? "",
      freeze_status: (contest.freeze_status as corev1.UpdateContestRequestModel.freeze_status) || "auto",
    },
    validate: {
      login: (value) => {
        const trimmed = value.trim();
        if (!trimmed) {
          return "Логин контеста обязателен";
        }
        if (trimmed.length < 3 || trimmed.length > 64) {
          return "Логин должен быть от 3 до 64 символов";
        }
        if (!/^[a-z0-9]+(-[a-z0-9]+)*$/.test(trimmed)) {
          return "Логин может содержать только строчные буквы, цифры и дефисы (без дефисов по краям)";
        }
        return null;
      },
      freeze_duration_minutes: (value) =>
        value !== "" && value !== undefined && value !== null && Number(value) < 0
          ? "Длительность заморозки не может быть отрицательной"
          : null,
    },
  });

  const handleSave = async (values: typeof form.values) => {
    setSaving(true);
    const freezeDuration =
      values.freeze_duration_minutes !== "" &&
      values.freeze_duration_minutes !== undefined &&
      values.freeze_duration_minutes !== null
        ? Number(values.freeze_duration_minutes)
        : null;

    const newLogin = values.login.trim();
    const payload: corev1.UpdateContestRequestModel = {
      ...values,
      login: newLogin !== contest.login ? newLogin : undefined,
      start_time: values.start_time ? new Date(values.start_time).toISOString() : null,
      end_time: values.end_time ? new Date(values.end_time).toISOString() : null,
      freeze_duration_minutes: freezeDuration,
      freeze_status: values.freeze_status as corev1.UpdateContestRequestModel.freeze_status,
    };
    const [error] = await api.updateContest({
      orgLogin: contest.organization_login,
      contestLogin: contest.login,
      requestBody: payload,
    });
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

    if (newLogin && newLogin !== contest.login) {
      router.push(`/${contest.organization_login}/contests/${newLogin}/settings`);
    } else {
      router.refresh();
    }
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
            label="Логин (URL)"
            placeholder="Введите уникальный идентификатор контеста"
            description={`Ссылка: /${contest.organization_login}/contests/${form.values.login || contest.login}`}
            required
            {...form.getInputProps("login")}
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

          <NumberInput
            label="Заморозка монитора (минут до окончания)"
            min={0}
            clampBehavior="strict"
            {...form.getInputProps("freeze_duration_minutes")}
          />

          <CustomSelect
            label="Режим заморозки"
            value={form.values.freeze_status}
            onChange={(value) => form.setFieldValue("freeze_status", value as corev1.UpdateContestRequestModel.freeze_status)}
            options={FREEZE_STATUS_OPTIONS}
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
};
