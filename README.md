# Gate149

A self-hosted competitive programming platform. Gate149 lets you author problems, run contests, judge submissions in a sandbox, and publish blog posts — all within a single deployable stack.

## Features

- **Problem Workshop** — Git-versioned problem authoring with test cases, checkers, validators, generators, and interactors stored in S3-compatible object storage
- **Contests** — Time-bound competitions with scoreboard, freeze support, penalty scoring, and flexible access control
- **Judging** — Asynchronous sandbox execution (`go-judge`) with real-time verdict delivery over WebSocket
- **Blogs** — MDX articles with math (KaTeX) and syntax highlighting
- **Users & Organizations** — GitHub-style hierarchy: organizations own problems and contests; teams group users for permission management

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.24+, `net/http`, OpenAPI (`oapi-codegen`), `sqlc` |
| Frontend | Next.js 15 (App Router), TypeScript, Mantine 8, TanStack Query |
| Database | PostgreSQL 14 (`pgx/v5`) |
| Cache | Valkey / Redis |
| Message broker | NATS JetStream |
| Auth | Custom Session Auth (Bcrypt) |
| Sandbox | `criyle/go-judge` (gRPC) |
| Object storage | SeaweedFS (S3-compatible) |
| Reverse proxy | Nginx |

## Repository Layout

```
gate/
├── backend/      # Go API, WebSocket, judge worker, migrations
├── frontend/     # Next.js 15 web application
├── contracts/    # OpenAPI specs and generated TypeScript/Go client code
└── deploy/       # Docker Compose configs for local, dev, and production
```

Each subdirectory has its own README with detailed documentation:

- [`backend/README.md`](backend/README.md) — architecture, environment variables, running, code generation, tests
- [`frontend/README.md`](frontend/README.md) — features, environment variables, scripts, component patterns
- [`contracts/README.md`](contracts/README.md) — OpenAPI codegen guide, TypeScript and Go usage
- [`deploy/README.md`](deploy/README.md) — environment overview, Makefile commands, SSL setup

## Quick Start

The fastest way to run the full stack locally is Docker Compose.

**Prerequisites:** Docker, Docker Compose

```bash
cp deploy/local/.env.example deploy/local/.env   # Fill in passwords and S3 credentials
make local-up
```

| Service | URL |
|---|---|
| Frontend | http://localhost:3000 |
| Backend API | http://localhost:8080 |
| Gateway (Nginx) | http://localhost:80 |

To stop: `make local-down`

## Development

### Backend

```bash
cd backend
go run . --migrate --env .env # Apply DB migrations
go run . --env .env           # Start the merged backend runtime
```

Run tests:

```bash
go test ./...                                              # Unit tests
go test -tags=integration ./tests/integration/...         # Integration tests (testcontainers)
```

### Frontend

```bash
cd frontend
bun install
bun dev      # Dev server on :3000
bun run build
bun run lint
```

### Contracts (code generation)

```bash
cd contracts
npm install
make all          # Generate Go server stubs + TypeScript clients
```
