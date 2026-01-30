/**
 * Централизованная цветовая схема приложения
 * Используется для консистентного цветового кодирования по типам контента
 */
export const APP_COLORS = {
  /** Задачи/проблемы */
  problems: 'blue',
  /** Контесты */
  contests: 'blue',
  /** Посылки/решения */
  submissions: 'green',
  /** Пользователи/профили */
  users: 'gray',
  /** Администрирование */
  admin: 'violet',
  /** Успешные действия, статусы */
  success: 'green',
  /** Цвета для различных действий */
  actions: {
    primary: 'blue',
    create: 'green',
    edit: 'blue',
    delete: 'red',
  },
} as const;
