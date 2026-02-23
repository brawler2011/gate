# Gate Deployment

Configuration and tools for deploying the Gate project.

## Environments

| Env      | Domain            | Purpose            | Notes |
|----------|-------------------|--------------------|-------|
| **local**| `localhost`       | Development        | HTTP only |
| **dev**  | `dev.gate149.ru`  | Staging/Testing    | HTTPS (Let's Encrypt) |
| **prod** | `gate149.ru`      | Production         | HTTPS, HSTS, Rate Limits |

## Usage

Use the provided `Makefile` for all common operations.

Format: `make [environment]-[command]`

### Common Commands

```bash
make local-up       # Start local environment
make dev-down       # Stop dev environment
make prod-logs      # View production logs
make local-restart  # Restart local containers
make dev-ps         # List running containers
```

### SSL (Dev/Prod)

```bash
make dev-ssl-init   # Initial setup (requires email)
make prod-ssl-renew # Renew certificate manually
```

### Build & Maintenance

```bash
make local-build    # Rebuild images for local
make build-all      # Rebuild all environments
make docker-prune   # Clean unused Docker resources
make prod-backup    # Backup production databases to ./backups/
```

## Directory Structure

*   `base/`: Shared configuration (Kratos, Judge0)
*   `local/`, `dev/`, `prod/`: Environment-specific configs (`docker-compose.yml`, `.env`)

## Requirements

*   Docker & Docker Compose
*   Make (optional, but recommended)
