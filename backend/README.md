# backend

[![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)](https://golang.org/)
[![OpenAPI v3](https://img.shields.io/badge/OpenAPI-v3-6BA81E?style=flat-square&logo=swagger&logoColor=white)](https://swagger.io/specification/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-336791?style=flat-square&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Valkey%2FRedis-A00?style=flat-square&logo=redis&logoColor=white)](https://valkey.io/)
[![NATS](https://img.shields.io/badge/NATS-27AAE1?style=flat-square)](https://nats.io/)
[![Docker](https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white)](https://www.docker.com/)

Go backend for a competitive programming platform. Handles problems, contests, submissions, judging, organizations, and teams. Authentication is delegated to [Ory Kratos](https://www.ory.sh/kratos/).

## Architecture overview

The backend now runs as a single long-lived process. One instance starts:

- the public REST API on `ADDRESS`
- the private Kratos webhook server on `PRIVATE_ADDRESS`
- the `/ws/submissions` WebSocket endpoint on the public server
- the async judge worker and outbox/submission consumers over **NATS JetStream**

Database migrations remain a separate one-shot mode.

### External dependencies

| Service | Role |
|---|---|
| PostgreSQL | Primary datastore |
| Valkey / Redis | Cache |
| NATS JetStream | Async messaging (submission events) |
| Ory Kratos | Identity & authentication |
| go-judge | Sandboxed code compilation and execution |
| S3-compatible storage (SeaweedFS) | Avatars, problem packages, blog images |

## Prerequisites

- Go 1.24+
- Docker (for running dependencies)
- [`sqlc`](https://sqlc.dev/) — SQL code generation (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`)
- [`oapi-codegen`](https://github.com/oapi-codegen/oapi-codegen) — OpenAPI handler generation

## Configuration

All configuration is read from environment variables (or a `.env` file passed via `--env`).

```dotenv
# General
ENV=dev                          # dev | local | prod
ADDRESS=0.0.0.0:13000            # Main API listen address
PRIVATE_ADDRESS=:13011           # Kratos webhook server address
ALLOWED_ORIGINS=http://localhost,http://127.0.0.1

# PostgreSQL
POSTGRES_DSN=                    # Full DSN (overrides individual vars below)
POSTGRES_USER=postgres
POSTGRES_PASSWORD=postgres
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_DB=gate
POSTGRES_SSLMODE=disable

# Redis / Valkey
REDIS_URL=                       # Full URL (overrides individual vars below)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB_INDEX=0

# NATS
NATS_URL=                        # Full URL (overrides individual vars below)
NATS_HOST=localhost
NATS_PORT=4222

# Ory Kratos
KRATOS_URL=http://localhost:4433
KRATOS_ADMIN_URL=http://localhost:4434

# S3-compatible storage
S3_ENDPOINT=                     # required
S3_ACCESS_KEY=                   # required
S3_SECRET_KEY=                   # required
# Region and bucket names are fixed in backend defaults for SeaweedFS

# go-judge
GOJUDGE_GRPC_ADDR=localhost:5051

# Judge worker
JUDGE_WORKER_COUNT=4
JUDGE_TEMP_DIR=/tmp/judge
JUDGE_TIMEOUT=300000             # ms
JUDGE_MAX_RETRIES=3

# Default admin account
ADMIN_USERNAME=admin
ADMIN_PASSWORD=admin
```

## Running

### 1. Start dependencies

```bash
docker compose up -d
```

### 2. Apply migrations

```bash
go run . --migrate --env .env
```

### 3. Start the backend

```bash
go run . --env .env
```

## Development

### Code generation

```bash
# Regenerate SQLC queries
sqlc generate

# Regenerate OpenAPI handlers (from contracts module)
make gen
```

### Tests

```bash
# Unit tests
go test ./...

# Integration tests (require Docker)
go test -v -tags=integration ./tests/integration/...

# Sandbox integration tests (require go-judge)
go test -v -tags=integration ./pkg/sandbox/...
```

### Docker

Build from the repository root (the Dockerfile expects a sibling `contracts/` directory):

```bash
docker build -f backend/Dockerfile -t backend .
```

## WebSocket API

Connect to `GET /ws/submissions` on the main backend server. Supported query parameters:

| Parameter | Description |
|---|---|
| `contestId` | Filter by contest |
| `userId` | Filter by user |
| `problemId` | Filter by problem |
| `language` | Filter by language |
| `since` | Sequence number to replay history from |

The server maintains a 10,000-event ring buffer, so clients can catch up on missed events after reconnecting.
