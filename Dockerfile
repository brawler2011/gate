FROM oven/bun:1.4 AS frontend
WORKDIR /workspace
COPY package.json bun.lock ./
RUN bun install --frozen-lockfile
COPY index.html vite.config.ts tsconfig.json postcss.config.cjs ./
COPY frontend/ ./frontend/
RUN bun run build

FROM golang:1.25-alpine AS builder
RUN apk add --no-cache git ca-certificates
WORKDIR /workspace
COPY go.mod go.sum ./
COPY backend/contracts/go.mod backend/contracts/go.sum ./backend/contracts/
RUN go mod download
COPY backend/ ./backend/
COPY languages.yaml ./
RUN --mount=type=cache,target=/go/pkg/mod/ CGO_ENABLED=0 go build -o /bin/backend ./backend

FROM scratch AS runner
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/backend /bin/backend
COPY --from=builder /workspace/languages.yaml /languages.yaml
COPY --from=frontend /workspace/dist /dist
WORKDIR /
ENTRYPOINT ["/bin/backend"]
