# backend

[![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)](https://golang.org/)
[![OpenAPI v3](https://img.shields.io/badge/OpenAPI-v3-6BA81E?style=flat-square&logo=swagger&logoColor=white)](https://swagger.io/specification/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-336791?style=flat-square&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Valkey%2FRedis-A00?style=flat-square&logo=redis&logoColor=white)](https://valkey.io/)
[![NATS](https://img.shields.io/badge/NATS-27AAE1?style=flat-square)](https://nats.io/)
[![Docker](https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white)](https://www.docker.com/)

Go backend for a competitive programming platform. Handles problems, contests, submissions, judging, organizations, and teams. Authentication is implemented locally using custom session tokens and bcrypt password hashing.

## Architecture overview

The backend now runs as a single long-lived process. One instance starts:

- the public REST API on `ADDRESS`
- the `/ws/submissions` WebSocket endpoint on the public server
- the async judge worker and outbox/submission consumers over **NATS JetStream**

Database migrations remain a separate one-shot mode.

### External dependencies

| Service | Role |
|---|---|
| PostgreSQL | Primary datastore |
| Valkey / Redis | Cache |
| NATS JetStream | Async messaging (submission events) |
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
task be:gen-sqlc

# Regenerate all code (OpenAPI, SQLC, Mocks)
task gen
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

## Workshop save and import recovery

The database manifest stores the authoritative metadata and complete YAML snapshot.
YAML, statement text, title and limits are repaired from that snapshot after a failed
projection update. Storage and database failures are returned to the caller.

Import, initialization and file edits persist a hidden `.pending-mutation` marker
before changing files. An interrupted operation blocks workspace reads and publication,
including after a server restart. The error identifies the operation to retry: repeat
the import, initialization, or update/delete for the same path. A retry may supply
corrected content; an import replaces the entire workspace, including removed files.
Other operations cannot clear the marker. It is removed only after all files and
metadata are consistent; failure to remove it also returns an error. The marker is
reserved, excluded from package exports and preserved by import cleanup.
File edits, deletion and compilation validate the existing YAML before changing
files. Unreadable legacy YAML leaves files untouched and creates no pending marker,
so a corrected package can still be imported to repair the workspace.

The PostgreSQL locker monitors its dedicated session and cancels the operation
context if the connection is lost. Manifest, limits, title and package writes use
that same guarded session, serialized with monitoring; a dead owner cannot commit
through the repository pool. Storage operations check ownership before and after
I/O, and marker removal requires live ownership. Lost sessions return an error even
if the callback returns success. Validation and solution testing hold the operation
lock through all reads and sandbox execution.

This protocol relies on cooperative context cancellation at storage boundaries.
It cannot stop arbitrary Go callbacks or fence an object-store request already sent
before session loss. A stronger guarantee would require immutable workspace
generations or conditional object writes. Direct database/object-store changes
outside the workspace API also bypass this protocol.

## Diagnostic logging

The backend writes sanitized JSON through `slog` to stdout in every environment.
There are no Gate OTel exporters or metrics providers. HTTP requests use safe route
patterns and `X-Request-ID`; persisted outbox/NATS headers correlate async judging.
