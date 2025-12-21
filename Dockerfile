FROM golang:1.24-alpine AS base
RUN apk add --no-cache git
WORKDIR /workspace

COPY contracts /workspace/contracts
COPY judge0-go-sdk /workspace/judge0-go-sdk
COPY pandoc-go-sdk /workspace/pandoc-go-sdk

WORKDIR /workspace/core
COPY core/go.mod core/go.sum ./
RUN go mod download -x

FROM base AS builder
WORKDIR /workspace/core
COPY core/ ./
RUN --mount=type=cache,target=/go/pkg/mod/ \
  go build -o /bin/core .

FROM scratch AS runner
COPY --from=builder /bin/core /bin/
ENTRYPOINT [ "/bin/core" ]