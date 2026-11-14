# Gate

[![CI](https://github.com/brawler2011/gate/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/brawler2011/gate/actions/workflows/ci.yml)
[![Deploy Production](https://github.com/brawler2011/gate/actions/workflows/deploy-prod.yml/badge.svg)](https://github.com/brawler2011/gate/actions/workflows/deploy-prod.yml)
[![License: AGPL-3.0](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)

## Dependencies

Install [mise](https://mise.jdx.dev) and [Docker](https://docs.docker.com/get-docker/), then from the repository root:

```sh
mise install                    # Go, Bun, Task and the other tools pinned in mise.toml
bun install --frozen-lockfile
task hooks:install
```

## Running locally

```sh
cp deploy/local/.env.example deploy/local/.env
task docker:up                  # PostgreSQL, Redis, NATS, SeaweedFS, go-judge
task be:migrate
task dev                        # backend on :8080, frontend on http://localhost:3000
```

Tooling tasks use `<area>:<action>[:<qualifier>]` (for example, `be:test` and
`contracts:gen`). Bare names `dev`, `test`, `gen`, `check`, and `clean` cover all
areas. Run `task --list` for commands; `task check` runs the required CI checks
except integration tests and image builds. The development and check scripts in
`package.json` keep their standard names (`dev`, `build`, `typecheck`, `test`, `lint`, `knip`).
