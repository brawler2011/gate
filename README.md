# Gate

![Go](https://img.shields.io/badge/Go-00ADD8?logo=go&logoColor=white)
![TypeScript](https://img.shields.io/badge/TypeScript-3178C6?logo=typescript&logoColor=white)
![React](https://img.shields.io/badge/React-20232A?logo=react&logoColor=61DAFB)
![PostgreSQL](https://img.shields.io/badge/PostgreSQL-4169E1?logo=postgresql&logoColor=white)
![Kubernetes](https://img.shields.io/badge/Kubernetes-326CE5?logo=kubernetes&logoColor=white)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)

## Dependencies

Install [mise](https://mise.jdx.dev), [Docker](https://docs.docker.com/get-docker/) (image builds and integration tests) and k3s ([Local k3s](deploy/README.md#local-k3s)), then from the repository root:

```sh
mise install                    # Go, Bun, Task and the other tools pinned in mise.toml
bun install --frozen-lockfile
task hooks:install
```

## Running locally

```sh
cp deploy/local/.env.example deploy/local/.env
task infra:up                   # PostgreSQL, Redis, NATS, SeaweedFS, go-judge in local k3s
task be:migrate
task dev                        # backend on :8080, frontend on http://localhost:3000
```
