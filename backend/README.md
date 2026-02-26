# backend

[![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)](https://golang.org/)
[![OpenAPI v3](https://img.shields.io/badge/OpenAPI-v3-6BA81E?style=flat-square&logo=swagger&logoColor=white)](https://swagger.io/specification/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-336791?style=flat-square&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Valkey%2FRedis-A00?style=flat-square&logo=redis&logoColor=white)](https://valkey.io/)
[![NATS](https://img.shields.io/badge/NATS-27AAE1?style=flat-square)](https://nats.io/)
[![Docker](https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white)](https://www.docker.com/)

Go backend for a competitive programming platform. Handles problems, contests, submissions, judging, organizations, and teams. Authentication is delegated to [Ory Kratos](https://www.ory.sh/kratos/).

## Architecture overview

The backend runs as several independent processes that communicate via **NATS JetStream**:

| Process | Command | Purpose |
|---|---|---|
| API server | `go run . server` | Main REST API |
| WebSocket server | `go run . ws` | Real-time submission events |
| Kratos webhook | `go run . kratos` | Private webhook for Ory Kratos user lifecycle |
| Judge worker | `go run . judge` | Async code judging via go-judge sandbox |
| Migrations | `go run . migrate` | Apply DB migrations (goose, embedded SQL) |

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
WS_ADDRESS=:8081                 # WebSocket server address

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
S3_REGION=us-east-1
S3_AVATAR_BUCKET=avatars
S3_PACKAGE_BUCKET=problem-packages
S3_BLOG_BUCKET=blog-images

# Problem workshop
WORKSHOP_REPOS_DIR=./workshop-repos

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
go run . migrate --env .env
```

### 3. Start processes

```bash
# Main REST API
go run . server --env .env

# WebSocket server (separate terminal)
go run . ws --env .env

# Kratos webhook server (separate terminal)
go run . kratos --env .env

# Judge worker (separate terminal)
go run . judge --env .env
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

Connect to `GET /ws/submissions` on the WebSocket server. Supported query parameters:

| Parameter | Description |
|---|---|
| `contestId` | Filter by contest |
| `userId` | Filter by user |
| `problemId` | Filter by problem |
| `language` | Filter by language |
| `since` | Sequence number to replay history from |

The server maintains a 10,000-event ring buffer, so clients can catch up on missed events after reconnecting.
