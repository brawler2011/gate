# Многоступенчатая сборка для оптимизации размера образа

# Этап 1: Сборка приложения
FROM oven/bun:1-alpine AS builder
WORKDIR /app

# Копируем файлы зависимостей и устанавливаем зависимости
COPY frontend/package.json frontend/bun.lock ./
# Кешируем зависимости bun для ускорения установки
RUN --mount=type=cache,id=bun-cache,target=/root/.bun/install/cache \
    bun install --frozen-lockfile

# Копируем остальные файлы frontend/ (включает contracts/)
COPY frontend/ .

# Переменные окружения для сборки
ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

ARG BACKEND_API_URL
ENV BACKEND_API_URL=$BACKEND_API_URL

ARG KRATOS_PUBLIC_URL
ENV KRATOS_PUBLIC_URL=$KRATOS_PUBLIC_URL

ARG NEXT_PUBLIC_APP_URL
ENV NEXT_PUBLIC_APP_URL=$NEXT_PUBLIC_APP_URL

ARG WEBSOCKET_URL
ENV WEBSOCKET_URL=$WEBSOCKET_URL

# Собираем приложение с использованием кэша Next.js
RUN --mount=type=cache,id=next-cache,target=/app/.next/cache \
    bun run next build

# Этап 2: Production образ
# Очень важно!!! Использовать oven/bun:1-slim Не alpine! Bun не оптимизирован под alpine
FROM oven/bun:1-slim AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

# Создаем пользователя для безопасности
RUN groupadd --system --gid 1001 nodejs && \
    useradd --system --uid 1001 -g nodejs nextjs

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
