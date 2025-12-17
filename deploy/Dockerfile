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

# Копируем остальные файлы frontend/ (включая .env.production, исключая папку deploy)
COPY frontend/ .
RUN rm -rf deploy

# Переменные окружения для сборки
# Только NEXT_PUBLIC_* переменные нужно передавать как build args
# Остальные (BACKEND_API_URL и т.д.) читаются из .env.production
ARG NEXT_PUBLIC_APP_URL

ENV NEXT_PUBLIC_APP_URL=$NEXT_PUBLIC_APP_URL
ENV NODE_ENV=production

# Отключаем телеметрию для небольшого ускорения
ENV NEXT_TELEMETRY_DISABLED=1

# Собираем приложение с использованием кэша Next.js
# Это позволяет не пересобирать неизмененные компоненты и страницы
RUN --mount=type=cache,id=next-cache,target=/app/.next/cache \
    bun run build

# Этап 2: Production образ
FROM oven/bun:1-alpine AS runner
WORKDIR /app

ENV NODE_ENV=production

# Создаем пользователя для безопасности
RUN addgroup --system --gid 1001 nodejs
RUN adduser --system --uid 1001 nextjs

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
