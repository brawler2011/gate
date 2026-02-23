# Local Development Environment

This is the local development environment configuration for the Gate project. It runs on `localhost` without SSL/HTTPS.

## Prerequisites

- Docker and Docker Compose installed
- Ports available: 80, 3000, 8080, 4433, 4434, 5432, 6379, 4222, 8222, 2358, 3030

## Quick Start

1. **Copy the environment file:**
   ```bash
   cp .env.example .env.local
   ```

2. **Update `.env.local` with actual passwords** (if you changed them from defaults)

3. **Start the environment:**
   ```bash
   docker-compose up -d
   ```

4. **Check status:**
   ```bash
   docker-compose ps
   ```

5. **View logs:**
   ```bash
   docker-compose logs -f
   ```

6. **Access the application:**
   - Frontend: http://localhost
   - Backend API: http://localhost/api
   - Kratos Admin: http://localhost:4434
   - Judge0: http://localhost:2358

## Services

### Core Services
- **Frontend** - Next.js application (port 3000, proxied via nginx)
- **Backend** - Go application (port 8080, proxied via nginx)
- **Nginx** - Reverse proxy (port 80)

### Authentication & Database
- **Kratos** - Identity and user management (ports 4433, 4434)
- **PostgreSQL** - Database server (port 5432)
  - Database: `app` - main application database
  - Database: `kratos` - Kratos identity database

### Infrastructure
- **Redis** - Cache and session storage (port 6379, DB 0)
- **NATS** - Message broker (ports 4222, 8222)
- **Judge0** - Code execution engine (port 2358)
- **Pandoc** - Document conversion (port 3030)

## Useful Commands

### Starting/Stopping
```bash
# Start all services
docker-compose up -d

# Stop all services
docker-compose down

# Stop and remove volumes (WARNING: deletes all data)
docker-compose down -v

# Restart a specific service
docker-compose restart backend
```

### Logs
```bash
# All logs
docker-compose logs -f

# Specific service logs
docker-compose logs -f backend
docker-compose logs -f frontend
docker-compose logs -f kratos
```

### Building
```bash
# Rebuild images
docker-compose build

# Rebuild specific service
docker-compose build backend

# Rebuild without cache
docker-compose build --no-cache
```

### Database
```bash
# Access PostgreSQL
docker-compose exec postgres psql -U postgres

# List databases
docker-compose exec postgres psql -U postgres -c "\l"

# Access app database
docker-compose exec postgres psql -U postgres -d app

# Run migrations (if you have a migrate command)
docker-compose exec backend ./core migrate
```

### Debugging
```bash
# Enter a container shell
docker-compose exec backend sh
docker-compose exec frontend sh

# Check container resource usage
docker stats

# Inspect a service
docker-compose exec backend env
```

## Development Workflow

1. **Make code changes** in your local editor
2. **Rebuild the service:**
   ```bash
   docker-compose build backend
   docker-compose up -d backend
   ```
3. **Check logs** for errors:
   ```bash
   docker-compose logs -f backend
   ```

## Troubleshooting

### Port Already in Use
```bash
# Find what's using the port
lsof -i :80  # or netstat -tulpn | grep :80

# Stop the conflicting service or change the port in docker-compose.yml
```

### Database Connection Issues
```bash
# Check if PostgreSQL is healthy
docker-compose exec postgres pg_isready -U postgres

# Recreate the database
docker-compose down
docker-compose up -d postgres
# Wait for it to be healthy, then start other services
docker-compose up -d
```

### Kratos Migration Failed
```bash
# Check Kratos migrate logs
docker-compose logs kratos-migrate

# Manually run migration
docker-compose run --rm kratos-migrate
```

### Reset Everything
```bash
# WARNING: This will delete all data
docker-compose down -v
docker-compose up -d
```

## Configuration Files

- `docker-compose.yml` - Service definitions
- `.env.local` - Environment variables (not committed to git)
- `.env.example` - Template for environment variables
- `nginx/default.conf` - Nginx configuration
- `kratos/kratos.yml` - Kratos configuration
- `init-db.sql` - Database initialization script

## Environment Variables

See `.env.example` for all available environment variables. Key variables:

- `ENV` - Environment name (local)
- `POSTGRES_DSN` - PostgreSQL connection string
- `REDIS_URL` - Redis connection string
- `KRATOS_PUBLIC_URL` - Kratos public API URL
- `JUDGE0_URL` - Judge0 API URL

## Network

All services run in the `gate-network-local` bridge network, allowing them to communicate using service names as hostnames.

## Volumes

Persistent data is stored in named volumes:
- `gate-postgres-data-local` - PostgreSQL data
- `gate-redis-data-local` - Redis data
- `gate-nats-data-local` - NATS data

## Notes

- This environment runs **without SSL** (HTTP only)
- Suitable for **local development only**
- All services use **default passwords** - change them for production!
- **Hot reload** may not work for all services - rebuild when needed
