# Gate

## Installation / Prerequisites

- [Task Installation](https://taskfile.dev/docs/installation)

## Diagnostics

Gate uses sanitized JSON backend logs and container log rotation (10 MB × 3).
The local Compose runs business dependencies; backend/frontend run separately.
See [logging and production transition runbook](docs/logging-runbook.md) for
correlation fields, troubleshooting, diagnostic losses and rollback/volume policy.
