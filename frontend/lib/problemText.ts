import type {ErrorCode, FieldProblem, Problem} from "@/contracts/core/v1/schema";

type Params = {[key: string]: unknown} | undefined;

const bound = (params: Params, key: string): number | undefined => {
  const value = params?.[key];
  return typeof value === "number" ? value : undefined;
};

// Genitive after "до", "не менее", "не более".
const characters = (n: number): string => (n % 10 === 1 && n % 100 !== 11 ? "символа" : "символов");

const lengthText = (params: Params): string => {
  const min = bound(params, "min");
  const max = bound(params, "max");
  if (min !== undefined && max !== undefined) {
    return `Длина должна быть от ${min} до ${max} ${characters(max)}`;
  }
  if (min !== undefined) {
    return `Не менее ${min} ${characters(min)}`;
  }
  if (max !== undefined) {
    return `Не более ${max} ${characters(max)}`;
  }
  return "Недопустимая длина";
};

const rangeText = (params: Params): string => {
  const min = bound(params, "min");
  const max = bound(params, "max");
  if (min !== undefined && max !== undefined) {
    return `Значение должно быть от ${min} до ${max}`;
  }
  if (min !== undefined) {
    return `Значение должно быть не меньше ${min}`;
  }
  if (max !== undefined) {
    return `Значение должно быть не больше ${max}`;
  }
  return "Значение вне допустимого диапазона";
};

const sizeText = (bytes: number): string => {
  if (bytes >= 1024 * 1024) {
    return `${Math.round(bytes / 1024 / 1024)} МБ`;
  }
  return bytes >= 1024 ? `${Math.round(bytes / 1024)} КБ` : `${bytes} байт`;
};

const tooLargeText = (params: Params): string => {
  const max = bound(params, "max");
  return max === undefined ? "Слишком большой размер" : `Размер не должен превышать ${sizeText(max)}`;
};

const limitText = (params: Params): string => {
  const limit = bound(params, "limit");
  return limit === undefined ? "Достигнут лимит" : `Достигнут лимит: ${limit}`;
};

const languageText = (prefix: string) => (params: Params): string => {
  const id = params?.id;
  return typeof id === "string" ? `${prefix}: ${id}` : prefix;
};

const text = (params: Params, key: string): string | undefined => {
  const value = params?.[key];
  return typeof value === "string" && value !== "" ? value : undefined;
};

// Appends the offending value to a message, as in "Неизвестное значение: cpp14".
const withValue = (message: string) => (params: Params): string => {
  const value = text(params, "value") ?? (typeof params?.value === "number" ? String(params.value) : undefined);
  return value ? `${message}: ${value}` : message;
};

const tooManyText = (params: Params): string => {
  const max = bound(params, "max");
  return max === undefined ? "Слишком много элементов" : `Не более ${max} элементов`;
};

const texts: Record<ErrorCode, string | ((params: Params) => string)> = {
  invalid: "Некорректные данные",
  unauthenticated: "Требуется вход в систему",
  forbidden: "Недостаточно прав",
  not_found: "Не найдено",
  conflict: "Конфликт с текущим состоянием данных. Обновите страницу и повторите",
  payload_too_large: "Слишком большой запрос или файл",
  rate_limited: "Слишком много запросов. Попробуйте позже",
  internal: "Ошибка сервера. Попробуйте позже",
  not_implemented: "Действие не поддерживается",
  concurrent_update: "Данные одновременно изменили. Повторите действие",
  in_use: "Объект используется и не может быть удалён",
  already_member: "Уже является участником",
  request_pending: "Заявка уже отправлена и ожидает рассмотрения",
  username_taken: "Имя пользователя уже занято",
  email_taken: "Email уже используется",
  organization_login_taken: "Логин организации уже занят",
  contest_login_taken: "Логин контеста уже занят",
  team_slug_taken: "Команда с таким названием уже существует",
  problem_short_name_taken: "Короткое имя задачи уже занято",
  problem_in_contest: "Задача уже добавлена в контест",
  email_not_verified: "Почта ещё не подтверждена",
  wrong_password: "Неверный пароль",
  invitation_pending: "Приглашение уже отправлено",
  request_resolved: "Заявка уже рассмотрена",
  invite_only: "Вступление только по приглашениям",
  not_org_member: "Сначала нужно вступить в организацию",
  link_invalid: "Ссылка недействительна или устарела. Запросите новую",
  account_expired: "Срок действия аккаунта истёк",
  temporary_account_unavailable: "Временный аккаунт недоступен: он истёк или уже привязан",
  own_account: "Нельзя привязать собственный аккаунт",
  no_email: "У пользователя не указан email",
  unsupported_image: "Неподдерживаемый формат изображения",
  owner_only: "Это может сделать только владелец",
  primary_owner_required: "Сначала назначьте нового основного владельца",
  owner_role_fixed: "Роль владельца нельзя изменить",
  own_role: "Нельзя изменить собственную роль",
  statements_hidden: "Условия задач скрыты",
  contest_not_started: "Контест ещё не начался",
  contest_ended: "Контест уже завершён",
  clarifications_disabled: "Вопросы жюри в этом контесте отключены",
  scoreboard_frozen: "Фильтр по результатам недоступен во время заморозки",
  limit_reached: limitText,
  unknown_language: languageText("Неизвестный язык"),
  language_not_allowed: languageText("Язык не разрешён в этом контесте"),
  package_not_ready: "У задачи нет собранного пакета",
  stale_revision: "Черновик изменился в другом окне. Обновите страницу",
  operation_running: "Операция ещё выполняется. Подождите",
  build_finished: "Сборка уже завершена",
  required: "Обязательное поле",
  length: lengthText,
  format: "Недопустимый формат",
  reserved: "Это значение зарезервировано",
  range: rangeText,
  end_before_start: "Окончание должно быть позже начала",
  duplicate: withValue("Значение повторяется"),
  too_large: tooLargeText,
  unknown: withValue("Неизвестное значение"),
  not_utf8: "Текст должен быть в кодировке UTF-8",
  cycle: "Зависимости образуют цикл",
  inconsistent: "Не согласуется с другими настройками задачи",
  too_many: tooManyText,
  unsupported: withValue("Не поддерживается"),
  zip_invalid: "Архив повреждён или содержит недопустимые пути",
  package_layout: "Неверная структура пакета",
  package_syntax: "Не удалось разобрать файл пакета",
};

// Codes that say less than the caller's description of the failed action.
const vague = new Set<string>(["invalid", "conflict", "internal", "not_implemented"]);

const fieldLabels: Record<string, string> = {
  "/title": "Название",
  "/name": "Название",
  "/login": "Логин",
  "/description": "Описание",
  "/username": "Имя пользователя",
  "/email": "Email",
  "/new_email": "Новый email",
  "/password": "Пароль",
  "/old_password": "Текущий пароль",
  "/new_password": "Новый пароль",
  "/role": "Роль",
  "/end_time": "Окончание",
  "/freeze_duration_minutes": "Заморозка",
  "/body": "Текст",
  "/question": "Вопрос",
  "/answer": "Ответ",
  "/count": "Количество",
  "/prefix": "Префикс",
  "/text": "Текст",
  "/preview_image": "Обложка",
  "/submission": "Решение",
};

const codeText = (code: string, params: Params): string => {
  const text = texts[code as ErrorCode] ?? texts.invalid;
  return typeof text === "function" ? text(params) : text;
};

const problemOf = (error: unknown): Problem | undefined => {
  if (typeof error !== "object" || error === null) {
    return undefined;
  }
  const candidate = error as Partial<Problem>;
  return typeof candidate.code === "string" && typeof candidate.status === "number" ? candidate as Problem : undefined;
};

export const fieldText = (field: FieldProblem): string => codeText(field.code, field.params);

/** User-facing text of an API error; fallback describes the failed action when the code is vague. */
export const problemMessage = (error: unknown, fallback?: string): string => {
  const problem = problemOf(error);
  if (!problem) {
    return fallback ?? (error instanceof Error && error.message ? error.message : codeText("internal", undefined));
  }
  if (problem.errors?.length) {
    return problem.errors.map((field) => {
      const label = fieldLabels[field.pointer];
      const text = fieldText(field);
      return label ? `${label}: ${text.charAt(0).toLowerCase()}${text.slice(1)}` : text;
    }).join(". ");
  }
  if (fallback && vague.has(problem.code)) {
    return fallback;
  }
  // Package diagnostics name the file they are about.
  const path = text(problem.params, "path");
  return path ? `${path}: ${codeText(problem.code, problem.params)}` : codeText(problem.code, problem.params);
};
