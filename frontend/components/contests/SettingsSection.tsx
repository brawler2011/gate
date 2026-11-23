import {Button, Stack, TextInput, NumberInput, Badge, Combobox, useCombobox, InputBase, Input, MultiSelect, Text, Divider, Switch, Group} from "@mantine/core";
import {useForm} from "@mantine/form";
import {notifications} from "@mantine/notifications";
import {useQueryClient} from "@tanstack/react-query";
import {useState} from "react";
import {useRevalidator, useNavigate} from "react-router-dom";

import {DownloadStatementsButton} from "@/components/contests/DownloadStatementsButton";
import {StatusMessage} from "@/components/shared/StatusMessage";
import {api} from "@/lib/api";
import {useLanguages} from "@/lib/languages";
import {problemMessage} from "@/lib/problemText";
import {APP_COLORS} from "@/lib/theme/colors";

import type * as corev1 from "@/contracts/core/v1/schema";
import type {ReactNode} from "react";

interface SettingsSectionProps {
  contest: corev1.ContestModel;
}

const SCOPE_OPTIONS = [
  {label: "Все", value: "public", color: "green"},
  {label: "Участник", value: "participant", color: "gray"},
  {label: "Модератор", value: "moderator", color: "yellow"},
];

const PHASED_SCOPES = [
  {key: "scoreboard", label: "Монитор"},
  {key: "submissions", label: "Список чужих посылок"},
  {key: "sources", label: "Чужой код"},
  {key: "reports", label: "Чужие протоколы тестирования"},
] as const;

const FEEDBACK_OPTIONS = [
  {label: "Скрыты", value: "hidden", color: "red"},
  {label: "Вердикт", value: "verdict", color: "yellow"},
  {label: "Протокол по примерам", value: "samples", color: "blue"},
  {label: "Протокол по всем тестам", value: "tests", color: "blue"},
  {label: "Полный, с данными теста", value: "full", color: "green"},
];

const VISIBILITY_OPTIONS = [
  {label: "Публичный", value: "public", color: "green"},
  {label: "Приватный", value: "private", color: "red"},
];

const JOIN_POLICY_OPTIONS = [
  {label: "Открытый (свободная регистрация)", value: "open", color: "green"},
  {label: "По заявкам (требуется одобрение)", value: "by_request", color: "blue"},
  {label: "Только по приглашению", value: "invite_only", color: "orange"},
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

const RESERVED_CONTEST_LOGINS = new Set([
  "problems",
  "teams",
  "members",
  "settings",
  "submit",
  "mysubmissions",
  "submissions",
  "monitor",
]);

export const SettingsSection = ({contest}: SettingsSectionProps): ReactNode => {
  const navigate = useNavigate();
  const revalidator = useRevalidator();
  const queryClient = useQueryClient();
  const [saving, setSaving] = useState(false);
  const catalog = useLanguages();
  const [statusMessage, setStatusMessage] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  const form = useForm<{
    login: string;
    title: string;
    description: string;
    visibility: string;
    scoreboard_during: corev1.ContestScope;
    scoreboard_after: corev1.ContestScope;
    submissions_during: corev1.ContestScope;
    submissions_after: corev1.ContestScope;
    sources_during: corev1.ContestScope;
    sources_after: corev1.ContestScope;
    reports_during: corev1.ContestScope;
    reports_after: corev1.ContestScope;
    feedback_during: corev1.FeedbackLevel;
    feedback_after: corev1.FeedbackLevel;
    freeze_hides_attempts: boolean;
    unfreeze_time: string;
    start_time: string;
    end_time: string;
    freeze_duration_minutes: number | string | undefined | null;
    freeze_status: corev1.UpdateContestRequestModel["freeze_status"];
    join_policy: corev1.UpdateContestRequestModel["join_policy"];
    enable_drafts: boolean;
    enable_upsolving: boolean;
    allow_virtual: boolean;
    hide_statements: boolean;
    languages: string[];
  }>({
    initialValues: {
      login: contest.login || "",
      title: contest.title,
      description: contest.description,
      visibility: contest.visibility,
      join_policy: contest.join_policy,
      enable_drafts: contest.enable_drafts,
      enable_upsolving: contest.enable_upsolving,
      allow_virtual: contest.allow_virtual,
      hide_statements: contest.hide_statements,
      languages: contest.languages,
      scoreboard_during: contest.scoreboard_during,
      scoreboard_after: contest.scoreboard_after,
      submissions_during: contest.submissions_during,
      submissions_after: contest.submissions_after,
      sources_during: contest.sources_during,
      sources_after: contest.sources_after,
      reports_during: contest.reports_during,
      reports_after: contest.reports_after,
      feedback_during: contest.feedback_during,
      feedback_after: contest.feedback_after,
      freeze_hides_attempts: contest.freeze_hides_attempts,
      unfreeze_time: toLocalDatetimeString(contest.unfreeze_time),
      start_time: toLocalDatetimeString(contest.start_time),
      end_time: toLocalDatetimeString(contest.end_time),
      freeze_duration_minutes: contest.freeze_duration_minutes ?? "",
      freeze_status: (contest.freeze_status as corev1.UpdateContestRequestModel["freeze_status"]) || "auto",
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
        if (RESERVED_CONTEST_LOGINS.has(trimmed)) {
          return `Логин '${trimmed}' зарезервирован`;
        }
        return null;
      },
      freeze_duration_minutes: (value) =>
        value !== "" && value !== undefined && value !== null && Number(value) < 1
          ? "Длительность заморозки должна быть не меньше минуты"
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
      unfreeze_time: values.unfreeze_time ? new Date(values.unfreeze_time).toISOString() : null,
      freeze_duration_minutes: freezeDuration,
      freeze_status: values.freeze_status as corev1.UpdateContestRequestModel["freeze_status"],
      join_policy: values.join_policy as corev1.UpdateContestRequestModel["join_policy"],
      enable_drafts: values.enable_drafts,
      enable_upsolving: values.enable_upsolving,
      allow_virtual: values.allow_virtual,
      hide_statements: values.hide_statements,
    };
    const {error} = await api.PATCH(
      "/organizations/{org_login}/contests/{contest_login}",
      {
        params: {
          path: {
            org_login: contest.organization_login,
            contest_login: contest.login,
          },
        },
        body: payload,
      },
    );
    setSaving(false);

    if (error) {
      console.error("Failed to update contest:", error);
      notifications.show({
        title: "Ошибка",
        message: problemMessage(error, "Не удалось обновить настройки"),
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
      navigate(`/${contest.organization_login}/${newLogin}/settings`);
    } else {
      void queryClient.invalidateQueries({
        queryKey: api.queryOptions(
          "get",
          "/organizations/{org_login}/contests/{contest_login}",
          {
            params: {
              path: {
                org_login: contest.organization_login,
                contest_login: contest.login,
              },
            },
          },
        ).queryKey,
      });
      void revalidator.revalidate();
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
            description={`Ссылка: /${contest.organization_login}/${form.values.login || contest.login}`}
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
            min={1}
            clampBehavior="strict"
            {...form.getInputProps("freeze_duration_minutes")}
          />

          <CustomSelect
            label="Режим заморозки"
            value={form.values.freeze_status ?? "auto"}
            onChange={(value) => form.setFieldValue("freeze_status", value as corev1.UpdateContestRequestModel["freeze_status"])}
            options={FREEZE_STATUS_OPTIONS}
          />

          <TextInput
            label="Разморозка монитора"
            description="Пусто — в момент окончания контеста"
            type="datetime-local"
            {...form.getInputProps("unfreeze_time")}
          />

          <Group justify="space-between">
            <Text size="sm">Скрывать попытки после заморозки</Text>
            <Switch
              checked={form.values.freeze_hides_attempts}
              onChange={(event) => form.setFieldValue("freeze_hides_attempts", event.currentTarget.checked)}
            />
          </Group>

          <CustomSelect
            label="Видимость"
            value={form.values.visibility}
            onChange={(value) => form.setFieldValue("visibility", value)}
            options={VISIBILITY_OPTIONS}
          />

          <CustomSelect
            label="Режим участия"
            value={form.values.join_policy ?? "open"}
            onChange={(value) => form.setFieldValue("join_policy", value as corev1.UpdateContestRequestModel["join_policy"])}
            options={JOIN_POLICY_OPTIONS}
            description="В открытом контесте любой авторизованный пользователь может отправлять решения"
          />

          <MultiSelect
            label="Языки"
            description="Конфигурации, в которых участники могут отправлять решения; пусто — все. Программы задач не ограничиваются"
            placeholder={form.values.languages.length === 0 ? "Все языки" : undefined}
            data={catalog.map((language) => ({value: language.id, label: language.name}))}
            value={form.values.languages}
            onChange={(value) => form.setFieldValue("languages", value)}
            clearable
            searchable
          />

          <Group justify="space-between" mt="xs">
            <Text size="sm">Разрешить черновики решений</Text>
            <Switch
              checked={form.values.enable_drafts}
              onChange={(event) => form.setFieldValue("enable_drafts", event.currentTarget.checked)}
            />
          </Group>

          <Group justify="space-between">
            <Text size="sm">Разрешить дорешивание после окончания</Text>
            <Switch
              checked={form.values.enable_upsolving}
              onChange={(event) => form.setFieldValue("enable_upsolving", event.currentTarget.checked)}
            />
          </Group>

          <Group justify="space-between">
            <Text size="sm">Разрешить виртуальное участие после окончания</Text>
            <Switch
              checked={form.values.allow_virtual}
              onChange={(event) => form.setFieldValue("allow_virtual", event.currentTarget.checked)}
            />
          </Group>

          <Group justify="space-between">
            <Stack gap={2}>
              <Text size="sm">Скрыть условия задач</Text>
              <Text size="xs" c="dimmed">
                Условия задач будут скрыты для участников на сайте (режим очной олимпиады с печатными условиями)
              </Text>
            </Stack>
            <Switch
              checked={form.values.hide_statements}
              onChange={(event) => form.setFieldValue("hide_statements", event.currentTarget.checked)}
            />
          </Group>

          <Divider my="sm" />

          <Group justify="space-between" align="center">
            <Stack gap={2}>
              <Text size="sm" fw={500}>Печатный буклет задач</Text>
              <Text size="xs" c="dimmed">
                Сгенерировать официальный PDF буклет со всеми условиями и примерами контеста (LaTeX / olymp.sty)
              </Text>
            </Stack>
            <DownloadStatementsButton
              orgLogin={contest.organization_login}
              contestLogin={contest.login}
            />
          </Group>

          <Divider my="sm" />

          <Text size="sm" fw={500}>Видимость результатов</Text>
          <Text size="xs" c="dimmed">
            Кто видит чужие результаты во время контеста и после окончания. Модераторы видят всё;
            чужие посылки никогда не показываются подробнее, чем их автору.
          </Text>

          {PHASED_SCOPES.map(({key, label}) => (
            <Group key={key} grow align="flex-start">
              <CustomSelect
                label={`${label}: во время`}
                value={form.values[`${key}_during`]}
                onChange={(value) => form.setFieldValue(`${key}_during`, value as corev1.ContestScope)}
                options={SCOPE_OPTIONS}
              />
              <CustomSelect
                label="после окончания"
                value={form.values[`${key}_after`]}
                onChange={(value) => form.setFieldValue(`${key}_after`, value as corev1.ContestScope)}
                options={SCOPE_OPTIONS}
              />
            </Group>
          ))}

          <Group grow align="flex-start">
            <CustomSelect
              label="Свои результаты: во время"
              value={form.values.feedback_during}
              onChange={(value) => form.setFieldValue("feedback_during", value as corev1.FeedbackLevel)}
              options={FEEDBACK_OPTIONS}
              description={"«Скрыты» — для очных олимпиад: участник видит только факт отправки, "
                + "монитор доступен лишь модераторам"}
            />
            <CustomSelect
              label="после окончания"
              value={form.values.feedback_after}
              onChange={(value) => form.setFieldValue("feedback_after", value as corev1.FeedbackLevel)}
              options={FEEDBACK_OPTIONS}
            />
          </Group>

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
