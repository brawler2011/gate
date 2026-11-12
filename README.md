# Gate

[![CI](https://github.com/brawler2011/gate/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/brawler2011/gate/actions/workflows/ci.yml)
[![Deploy Production](https://github.com/brawler2011/gate/actions/workflows/deploy-prod.yml/badge.svg)](https://github.com/brawler2011/gate/actions/workflows/deploy-prod.yml)

## Dependencies

Install [mise](https://mise.jdx.dev) and [Docker](https://docs.docker.com/get-docker/), then from the repository root:

```sh
mise install                    # Go, Bun, Task and the other tools pinned in mise.toml
bun install --frozen-lockfile
task install:hooks
```

## Running locally

```sh
cp deploy/local/.env.example deploy/local/.env
task docker:up                  # PostgreSQL, Redis, NATS, SeaweedFS, go-judge
task migrate:be
task dev                        # backend on :8080, frontend on http://localhost:3000
```
