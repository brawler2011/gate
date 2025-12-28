# Многоступенчатая сборка для оптимизации размера образа

# Этап 1: Сборка приложения
FROM oven/bun:1-alpine AS builder
WORKDIR /app

# Копируем файлы зависимостей и устанавливаем зависимости
COPY frontend/package.json frontend/bun.lock ./
# Кешируем зависимости bun для ускорения установки
RUN --mount=type=cache,id=bun-cache,target=/root/.bun/install/cache \
    bun install --frozen-lockfile

# Копируем contracts/ (нужны для импортов)
COPY contracts/ ../contracts/

# Переменные окружения для сборки (читаем из .env.production внутри контейнера)
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# Копируем остальные файлы frontend/
COPY frontend/ .

# Собираем приложение с использованием кэша Next.js
# Это позволяет не пересобирать неизмененные компоненты и страницы
RUN --mount=type=cache,id=next-cache,target=/app/.next/cache \
    bun run build

# Этап 2: Production образ
FROM oven/bun:1-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# Создаем пользователя для безопасности
RUN addgroup --system --gid 1001 nodejs && \
    adduser --system --uid 1001 nextjs

# Копируем необходимые файлы из builder
COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

# Меняем владельца файлов
RUN chown -R nextjs:nodejs /app

USER nextjs

EXPOSE 3000

ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

# Запускаем приложение
CMD ["bun", "server.js"]
