FROM golang:1.24-alpine AS base
RUN apk add --no-cache git
WORKDIR /workspace

# Копируем зависимости (локальные модули)
COPY contracts /workspace/contracts
COPY judge0-go-sdk /workspace/judge0-go-sdk
COPY pandoc-go-sdk /workspace/pandoc-go-sdk

# Копируем tester модуль
WORKDIR /workspace/tester
COPY tester/go.mod tester/go.sum ./
RUN go mod download -x

FROM base AS builder
WORKDIR /workspace/tester
COPY tester/ ./
RUN --mount=type=cache,target=/go/pkg/mod/ \
  go build -o /bin/server .

FROM scratch AS runner
COPY --from=builder /bin/server /bin/
ENTRYPOINT [ "/bin/server" ]