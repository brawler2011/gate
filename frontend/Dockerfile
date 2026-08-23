FROM oven/bun:1.4-slim AS builder
WORKDIR /app

COPY package.json bun.lock ./

RUN --mount=type=cache,id=bun-cache,target=/root/.bun/install/cache \
  bun install

COPY . .

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

ARG BACKEND_API_URL
ENV BACKEND_API_URL=$BACKEND_API_URL

ARG NEXT_PUBLIC_BACKEND_API_URL
ENV NEXT_PUBLIC_BACKEND_API_URL=$NEXT_PUBLIC_BACKEND_API_URL

ARG NEXT_PUBLIC_APP_URL
ENV NEXT_PUBLIC_APP_URL=$NEXT_PUBLIC_APP_URL

ARG NEXT_PUBLIC_WEBSOCKET_URL
ENV NEXT_PUBLIC_WEBSOCKET_URL=$NEXT_PUBLIC_WEBSOCKET_URL

ARG NEXT_PUBLIC_OTEL_SERVICE_NAME
ENV NEXT_PUBLIC_OTEL_SERVICE_NAME=$NEXT_PUBLIC_OTEL_SERVICE_NAME

ARG NEXT_PUBLIC_OTEL_EXPORTER_URL
ENV NEXT_PUBLIC_OTEL_EXPORTER_URL=$NEXT_PUBLIC_OTEL_EXPORTER_URL

RUN --mount=type=cache,id=next-cache,target=/app/.next/cache \
  bun run next build

FROM oven/bun:1.4-slim AS runner
WORKDIR /app

ENV NODE_ENV=production
ENV NEXT_TELEMETRY_DISABLED=1

RUN groupadd --system --gid 1001 nodejs && \
  useradd --system --uid 1001 -g nodejs nextjs

COPY --from=builder /app/public ./public
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static

RUN chown -R nextjs:nodejs /app

USER nextjs

EXPOSE 3000

ENV PORT=3000
ENV HOSTNAME="0.0.0.0"

CMD ["bun", "server.js"]
