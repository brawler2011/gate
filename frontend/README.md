# Gate149 Frontend

![Next.js](https://img.shields.io/badge/Next.js-15.5.9-black?logo=next.js)
![React](https://img.shields.io/badge/React-18.3.1-blue?logo=react)
![Mantine](https://img.shields.io/badge/Mantine-8.3.10-blue?logo=mantine)
![TypeScript](https://img.shields.io/badge/TypeScript-5.9.3-blue?logo=typescript)
![KaTeX](https://img.shields.io/badge/KaTeX-0.16.27-blue)
[![OpenAPI v3](https://img.shields.io/badge/OpenAPI-v3-green)](https://swagger.io/specification/)

Frontend для платформы Gate149 — системы для проведения соревнований по спортивному программированию. Построен на современных веб-технологиях с упором на Server-Side Rendering (SSR) и type-safety.

## 🎯 Возможности платформы

- **Задачи**: Создание, просмотр и решение задач по программированию с автоматической проверкой
- **Контесты**: Организация соревнований с несколькими задачами, системой подсчета очков и рейтинга
- **Посылки**: Real-time мониторинг статуса решений через WebSocket
- **Блоги**: Публикация статей с поддержкой MDX, математических формул (KaTeX) и подсветки кода
- **Управление пользователями**: Аутентификация через Ory Kratos, роли (admin/user), управление правами доступа к контестам
- **Мастерская**: Личный кабинет с созданными задачами и контестами
- **Профили**: Статистика и история решений пользователей

## 🏗️ Архитектура

### Server-Side Rendering First

Проект использует **SSR-first** подход:
- Все страницы по умолчанию — серверные компоненты
- Клиентские компоненты (`"use client"`) используются только для интерактивности
- Данные загружаются на сервере через `Call` wrapper
- Оптимизированная производительность и SEO

### OpenAPI-Generated Client

API клиент автоматически генерируется из OpenAPI спецификации:
- **Gateway API**: Единый клиент для всех микросервисов (`@contracts/gateway/v1`)
- Routing через NGINX к соответствующим бэкенд-сервисам (blogs, core/tester)
- Type-safe методы и модели

### Аутентификация

- **Ory Kratos**: Identity & User Management
- Session-based authentication с cookies
- Proxy через Next.js rewrites (`/api/.ory/*`)
- Защищенные маршруты с проверкой прав доступа

## 📁 Структура проекта

```
frontend/
├── app/                    # Next.js App Router
│   ├── admin/             # Админ-панель
│   ├── auth/              # Логин и регистрация
│   ├── blog/              # Блог-посты
│   ├── contests/          # Контесты, задачи, посылки
│   ├── problems/          # Просмотр и редактирование задач
│   ├── submissions/       # История посылок
│   ├── users/             # Профили пользователей
│   ├── workshop/          # Мастерская (личный кабинет)
│   └── layout.tsx         # Главный layout с header/footer
├── components/            # React компоненты
│   ├── AdminPage/         # Компоненты админ-панели
│   ├── ContestsPage/      # Компоненты страницы контестов
│   ├── ContestManage/     # Управление контестом
│   ├── Problem/           # Отображение условия задачи
│   ├── SubmissionsList/   # Список посылок с WebSocket
│   ├── Header/            # Навигация
│   ├── Footer/            # Подвал сайта
│   └── ...
├── lib/                   # Утилиты и хелперы
│   ├── api.ts             # Call wrapper для Gateway API
│   ├── auth.ts            # getCurrentUser, isAuthenticated
│   ├── permissions.ts     # Проверка прав доступа
│   ├── contest-role.ts    # Роли участников контеста
│   ├── formatDate.ts      # Форматирование дат
│   └── useSubmissionsWebSocket.ts  # WebSocket хук
└── public/                # Статические файлы
```

## ⚙️ Переменные окружения

Создайте файл `.env.local`:

```bash
# Backend Gateway API (через него также доступен Kratos по /api/.ory/)
BACKEND_API_URL=http://localhost:8080

# WebSocket для real-time обновлений посылок
WEBSOCKET_URL=ws://localhost:8080
```

### Production

```bash
BACKEND_API_URL=https://gate149.ru
WEBSOCKET_URL=wss://gate149.ru
```

## 🚀 Установка и запуск

### Предварительные требования

- **Node.js**: 18+
- **bun**: Последняя версия
- **Backend API**: Запущенный Go-сервер (core) на порту 8080
- **Ory Kratos**: Запущенный на порту 4433

### Установка зависимостей

```bash
cd frontend
bun install
```

### Скрипты

**Запуск dev-сервера:**

```bash
bun dev
```

Приложение будет доступно на `http://localhost:3000`

**Сборка для production:**

```bash
bun run build
```

**Запуск production сервера:**

```bash
bun run start
```

**Линтинг:**

```bash
bun run lint
```

## 🧩 Ключевые зависимости

### Фреймворки и библиотеки

- **next** (15.5.9): React framework с SSR, App Router, Server Actions
- **react** (18.3.1): UI библиотека
- **@mantine/core** (8.3.10): UI component library (buttons, forms, modals, tables)
- **@mantine/hooks**: Полезные хуки (useDisclosure, useMediaQuery и т.д.)
- **@mantine/form**: Управление формами
- **@mantine/dropzone**: Загрузка файлов
- **@mantine/notifications**: Toast уведомления

### Аутентификация и API

- **@ory/client** (1.22.15): Клиент для Ory Kratos
- **jsonwebtoken**: JWT токены
- **@contracts/gateway/v1**: Сгенерированный OpenAPI клиент (см. `/contracts`)

### Контент и отображение

- **katex** (0.16.27): Рендеринг математических формул (LaTeX)
- **react-markdown**: Markdown рендеринг
- **react-syntax-highlighter**: Подсветка синтаксиса кода
- **@next/mdx**: MDX поддержка для блога
- **next-mdx-remote**: Динамический рендеринг MDX
- **remark-math**, **rehype-katex**: Плагины для математики в Markdown
- **remark-gfm**: GitHub Flavored Markdown

### Утилиты и типы

- **typescript** (5.9.3): Type safety
- **@tabler/icons-react**: Иконки
- **postcss**: CSS обработка с Mantine presets

## 🎨 Технологии UI/UX

### Стилизация

- **CSS Modules**: Основной способ стилизации (`.module.css`)
- **Mantine Components**: Props-based styling через Mantine API
- **PostCSS**: С Mantine presets для CSS переменных

### Соглашения по стилям

```tsx
// Импорт CSS Module
import classes from './styles.module.css';

// Использование
<div className={classes.container}>
  <Title className={classes.title}>Заголовок</Title>
</div>
```

### Mantine Theme

Кастомная тема в `/lib/theme/theme.ts`:
- Цветовая схема
- Компонентные стили по умолчанию
- Responsive breakpoints

## 🔧 Ключевые концепции

### Server Components vs Client Components

**Server Components (по умолчанию):**
```tsx
// app/problems/page.tsx
export default async function ProblemsPage() {
  const [error, problems] = await Call(client => client.problems.listProblems());
  // Рендеринг на сервере, данные загружены до отправки HTML
  return <ProblemsList problems={problems} />;
}
```

**Client Components (для интерактивности):**
```tsx
'use client';
// Используется для форм, модальных окон, WebSocket и т.д.
import { useState } from 'react';

export function SubmissionForm() {
  const [code, setCode] = useState('');
  // ...
}
```

### API интеграция через Call

```tsx
import { Call } from '@/lib/api';

// Server-side (в Server Components, Server Actions)
const [error, data] = await Call(client => client.problems.getProblem({ id }));

if (error) {
  // Обработка ошибки
  return <ErrorDisplay error={error} />;
}

// Использование data
return <Problem data={data} />;
```

### Аутентификация

```tsx
import { getCurrentUser } from '@/lib/auth';

// Получение текущего пользователя (в Server Component)
const user = await getCurrentUser();

if (!user) {
  redirect('/auth/login');
}

// Проверка роли
if (user.role !== 'admin') {
  return <AccessDenied />;
}
```

### WebSocket для Real-time обновлений

```tsx
'use client';
import { useSubmissionsWebSocket } from '@/lib/useSubmissionsWebSocket';

export function SubmissionsList() {
  const submissions = useSubmissionsWebSocket(contestId);
  
  return (
    <Table>
      {submissions.map(s => <SubmissionRow key={s.id} submission={s} />)}
    </Table>
  );
}
```

## 📝 Разработка

### Добавление новой страницы

1. Создайте файл в `/app/[route]/page.tsx`
2. Используйте Server Component по умолчанию
3. Загружайте данные через `Call`
4. Добавьте ссылку в навигацию (Header)

### Создание нового компонента

1. Создайте папку в `/components/ComponentName/`
2. Создайте `ComponentName.tsx` и `index.ts`
3. При необходимости добавьте `styles.module.css`
4. Экспортируйте через `index.ts`

### Работа с формами

```tsx
import { useForm } from '@mantine/form';
import { TextInput, Button } from '@mantine/core';

export function MyForm() {
  const form = useForm({
    initialValues: { name: '' },
    validate: {
      name: (value) => value.length < 2 ? 'Слишком короткое имя' : null,
    },
  });

  const handleSubmit = async (values: typeof form.values) => {
    // Отправка данных
  };

  return (
    <form onSubmit={form.onSubmit(handleSubmit)}>
      <TextInput label="Имя" {...form.getInputProps('name')} />
      <Button type="submit">Отправить</Button>
    </form>
  );
}
```

## 🐛 Отладка

### Проверка переменных окружения

При старте приложения в консоли выводятся все env variables:
```
🔧 BACKEND_API_URL = http://localhost:8080
...
```

### Логи API запросов

API errors логируются в консоль сервера через `Call` wrapper.

### React DevTools

Установите расширение React DevTools для браузера для отладки компонентов.

## 📦 Docker

Проект поддерживает Docker deployment с `standalone` output:

```bash
docker build -t gate149-frontend .
docker run -p 3000:3000 gate149-frontend
```

См. `Dockerfile` и `docker-compose.yml` для деталей.

## 🤝 Contributing

1. Перед написанием нового кода изучите существующие компоненты
2. Следуйте паттернам проекта (SSR-first, CSS Modules, Mantine)
3. Весь UI текст на русском языке
4. Документируйте сложную логику
5. Type safety: никаких `any`, `!`, `as` без необходимости

## 📄 Лицензия

См. LICENSE файл в корне проекта.

## 🔗 Связанные репозитории

- **Core Backend**: `/core` - Go-сервер с бизнес-логикой
- **Contracts**: `/contracts` - OpenAPI спецификации и генерация клиентов
- **Infrastructure**: `/infr` - Docker Compose, Nginx, Kratos configs
