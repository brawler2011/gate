FROM ghcr.io/jdx/mise:2026.10.25-debian AS toolchain
WORKDIR /workspace
COPY mise.toml ./
RUN mise trust && mise install go bun

FROM toolchain AS frontend
COPY package.json bun.lock ./
RUN mise exec go bun -- bun install --frozen-lockfile
COPY index.html vite.config.ts tsconfig.json postcss.config.cjs ./
COPY frontend/ ./frontend/
RUN mise exec go bun -- bun run build

FROM toolchain AS builder
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/root/go/pkg/mod/ mise exec go bun -- go mod download
COPY backend/ ./backend/
COPY languages.yaml ./
COPY deploy/common/problem-policy.yaml ./problem-policy.yaml
RUN --mount=type=cache,target=/root/go/pkg/mod/ \
    --mount=type=cache,target=/root/.cache/go-build/ \
    mise exec go bun -- env CGO_ENABLED=0 go build -o /bin/backend ./backend

FROM scratch AS runner
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /bin/backend /bin/backend
COPY --from=builder /workspace/languages.yaml /languages.yaml
COPY --from=builder /workspace/problem-policy.yaml /problem-policy.yaml
COPY --from=frontend /workspace/dist /dist
WORKDIR /
ENTRYPOINT ["/bin/backend"]
