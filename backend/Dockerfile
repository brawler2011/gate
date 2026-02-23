FROM golang:1.24-alpine AS base
RUN apk add --no-cache git
WORKDIR /workspace

COPY contracts /workspace/contracts

WORKDIR /workspace/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download -x

FROM base AS builder
WORKDIR /workspace/backend
COPY backend/ ./
RUN --mount=type=cache,target=/go/pkg/mod/ \
  go build -o /bin/backend .

FROM scratch AS runner
COPY --from=builder /bin/backend /bin/
ENTRYPOINT [ "/bin/backend" ]