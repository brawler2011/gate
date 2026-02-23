# Production Environment (gate149.ru)

This is the **production** environment configuration for the Gate project. It runs on `gate149.ru` with HTTPS and includes production-grade optimizations.

## Prerequisites

- Docker and Docker Compose installed
- Domain `gate149.ru` pointing to your server
- Ports available: 80, 443
- SSL certificates (can be obtained with certbot)
- Sufficient resources (see resource limits below)

## Quick Start

1. **Copy the environment file:**
   ```bash
   cp .env.example .env.prod
   ```

2. **Update `.env.prod` with actual passwords** (if you changed them from defaults)

3. **Obtain SSL certificate (first time only):**
   ```bash
   # Start nginx temporarily for certificate validation
   docker-compose up -d nginx
   
   # Obtain certificate
   docker-compose run --rm certbot certonly --webroot \
     --webroot-path=/var/www/certbot \
     --email your-email@example.com \
     --agree-tos \
     --no-eff-email \
     -d gate149.ru
   
   # Restart nginx to apply certificate
   docker-compose restart nginx
   ```

4. **Start the environment:**
   ```bash
   docker-compose up -d
   ```

5. **Check status:**
   ```bash
   docker-compose ps
   ```

6. **View logs:**
   ```bash
   docker-compose logs -f
   ```

7. **Access the application:**
   - Frontend: https://gate149.ru
   - Backend API: https://gate149.ru/api
   - Health check: https://gate149.ru/health

## Services

### Core Services
- **Frontend** - Next.js application (production build)
- **Backend** - Go application (production mode)
- **Nginx** - Reverse proxy with SSL, rate limiting, caching

### Authentication & Database
- **Kratos** - Identity and user management (production mode, no debug logs)
- **PostgreSQL** - Database server (with resource limits)
  - Database: `prod_app` - main application database
  - Database: `prod_kratos` - Kratos identity database

### Infrastructure
- **Redis** - Cache and session storage (DB 2, with AOF persistence, LRU eviction)
- **NATS** - Message broker
- **Judge0** - Code execution engine
- **Pandoc** - Document conversion

### SSL/TLS
- **Certbot** - SSL certificate management (manual, via profiles)
- **Certbot-renew** - Automatic certificate renewal (every 12h)

## Resource Limits

Production deployment includes resource limits to prevent resource exhaustion:

| Service   | CPU Limit | Memory Limit | CPU Reserved | Memory Reserved |
|-----------|-----------|--------------|--------------|-----------------|
| Backend   | 2 cores   | 2GB          | 0.5 cores    | 512MB           |
| Frontend  | 1 core    | 1GB          | 0.25 cores   | 256MB           |
| Postgres  | 2 cores   | 2GB          | 0.5 cores    | 512MB           |
| Kratos    | 1 core    | 1GB          | 0.25 cores   | 256MB           |
| Redis     | 0.5 cores | 512MB        | 0.1 cores    | 128MB           |
| Judge0    | 2 cores   | 2GB          | 0.5 cores    | 512MB           |
| Pandoc    | 1 core    | 1GB          | 0.1 cores    | 128MB           |
| NATS      | 0.5 cores | 512MB        | 0.1 cores    | 128MB           |
| Nginx     | 0.5 cores | 256MB        | 0.1 cores    | 64MB            |

**Total recommended:** 8+ CPU cores, 16GB+ RAM

## Useful Commands

### Starting/Stopping
```bash
# Start all services
docker-compose up -d

# Stop all services
docker-compose down

# Restart a specific service
docker-compose restart backend

# View running containers
docker-compose ps
```

### Logs
```bash
# All logs
docker-compose logs -f

# Specific service logs
docker-compose logs -f backend
docker-compose logs -f nginx

# Last 100 lines
docker-compose logs --tail=100 backend
```

### Building & Updating
```bash
# Pull latest code and rebuild
git pull origin main
docker-compose build --no-cache
docker-compose up -d

# Rebuild specific service
docker-compose build backend
docker-compose up -d backend
```

### Database Management

#### Backup
```bash
# Backup prod_app database
docker-compose exec postgres pg_dump -U postgres prod_app > backup_app_$(date +%Y%m%d_%H%M%S).sql

# Backup prod_kratos database
docker-compose exec postgres pg_dump -U postgres prod_kratos > backup_kratos_$(date +%Y%m%d_%H%M%S).sql

# Backup all databases
docker-compose exec postgres pg_dumpall -U postgres > backup_all_$(date +%Y%m%d_%H%M%S).sql
```

#### Restore
```bash
# Restore prod_app database
docker-compose exec -T postgres psql -U postgres prod_app < backup_app.sql

# Restore prod_kratos database
docker-compose exec -T postgres psql -U postgres prod_kratos < backup_kratos.sql
```

#### Access Database
```bash
# PostgreSQL CLI
docker-compose exec postgres psql -U postgres

# Connect to prod_app
docker-compose exec postgres psql -U postgres -d prod_app

# List databases
docker-compose exec postgres psql -U postgres -c "\l"

# List tables in prod_app
docker-compose exec postgres psql -U postgres -d prod_app -c "\dt"
```

### SSL Certificates

#### Initial Certificate
```bash
# Obtain certificate for gate149.ru
docker-compose run --rm certbot certonly --webroot \
  --webroot-path=/var/www/certbot \
  --email admin@gate149.ru \
  --agree-tos \
  --no-eff-email \
  -d gate149.ru

# Restart nginx to apply
docker-compose restart nginx
```

#### Manual Renewal
```bash
# Renew certificate manually
docker-compose run --rm certbot renew

# Restart nginx after renewal
docker-compose restart nginx
```

#### Check Certificate Status
```bash
# List certificates and expiry dates
docker-compose run --rm certbot certificates

# Test certificate renewal (dry run)
docker-compose run --rm certbot renew --dry-run
```

Note: Auto-renewal runs via `certbot-renew` service every 12 hours.

### Monitoring

#### Health Checks
```bash
# Check all health statuses
docker-compose ps

# Test specific endpoints
curl https://gate149.ru/health
curl https://gate149.ru/health/alive
curl https://gate149.ru/api/health
```

#### Resource Usage
```bash
# Real-time resource usage
docker stats

# Container resource limits
docker-compose config | grep -A 5 "resources:"
```

#### Logs Analysis
```bash
# Search for errors in backend
docker-compose logs backend | grep -i error

# Check nginx access log
docker-compose exec nginx tail -f /var/log/nginx/access.log

# Check nginx error log
docker-compose exec nginx tail -f /var/log/nginx/error.log
```

### Maintenance

#### Rolling Updates (Zero Downtime)
```bash
# Update backend with zero downtime
docker-compose build backend
docker-compose up -d --no-deps --scale backend=2 backend
sleep 10
docker-compose up -d --no-deps --scale backend=1 backend

# Update frontend similarly
docker-compose build frontend
docker-compose up -d --no-deps --scale frontend=2 frontend
sleep 10
docker-compose up -d --no-deps --scale frontend=1 frontend
```

#### Database Migrations
```bash
# Run migrations on backend
docker-compose exec backend ./core migrate

# Check migration status
docker-compose exec backend ./core migrate status
```

#### Clean Up
```bash
# Remove unused images
docker image prune -a

# Remove unused volumes (CAREFUL!)
docker volume prune

# Clean everything (VERY CAREFUL!)
docker system prune -a --volumes
```

## Production Security Checklist

- [ ] SSL certificates installed and auto-renewing
- [ ] HSTS enabled (already configured)
- [ ] Security headers configured (already configured)
- [ ] Rate limiting enabled (already configured)
- [ ] Database passwords changed from defaults
- [ ] Redis password changed from defaults
- [ ] Kratos admin API access restricted (configure IP whitelist)
- [ ] Regular database backups scheduled
- [ ] Monitoring and alerting configured
- [ ] Firewall rules configured (only ports 80, 443 exposed)
- [ ] `.env.prod` not committed to git
- [ ] Log rotation configured
- [ ] Resource limits appropriate for server capacity

## Performance Optimizations

This production configuration includes:

1. **Nginx:**
   - Gzip compression for text assets
   - Static asset caching (7 days)
   - HTTP/2 enabled
   - Connection keepalive
   - Buffer optimizations
   - Open file cache

2. **Redis:**
   - AOF persistence for durability
   - LRU eviction policy
   - 512MB memory limit
   - Separate DB (2) from dev

3. **PostgreSQL:**
   - Connection pooling (max 20 connections)
   - Optimized for production use

4. **Rate Limiting:**
   - API endpoints: 10 req/s (burst 20)
   - Auth endpoints: 5 req/s (burst 10)

5. **Resource Limits:**
   - CPU and memory limits prevent resource exhaustion
   - Guaranteed minimums for critical services

## Troubleshooting

### High CPU Usage
```bash
# Check which container is using CPU
docker stats

# Check backend logs for issues
docker-compose logs --tail=100 backend

# Restart specific service
docker-compose restart backend
```

### High Memory Usage
```bash
# Check memory usage
docker stats

# Check Redis memory
docker-compose exec redis redis-cli --pass fZJrpQtdzJ6XMrqB INFO memory

# Clear Redis cache if needed
docker-compose exec redis redis-cli --pass fZJrpQtdzJ6XMrqB FLUSHDB
```

### Database Connection Issues
```bash
# Check database health
docker-compose exec postgres pg_isready -U postgres

# Check active connections
docker-compose exec postgres psql -U postgres -c "SELECT count(*) FROM pg_stat_activity;"

# Restart database (CAUTION)
docker-compose restart postgres
```

### SSL Certificate Issues
```bash
# Verify certificate files
docker-compose exec nginx ls -la /etc/letsencrypt/live/gate149.ru/

# Test nginx configuration
docker-compose exec nginx nginx -t

# Check certificate expiry
docker-compose run --rm certbot certificates

# Force renewal
docker-compose run --rm certbot renew --force-renewal
docker-compose restart nginx
```

### Application Not Responding
```bash
# Check all services
docker-compose ps

# Check nginx logs
docker-compose logs --tail=50 nginx

# Check if services are healthy
docker-compose exec backend wget -O- http://localhost:8080/health
docker-compose exec frontend wget -O- http://localhost:3000

# Restart all services
docker-compose restart
```

### Out of Disk Space
```bash
# Check disk usage
df -h

# Check Docker disk usage
docker system df

# Clean up old images/containers
docker system prune -a

# Clean logs
truncate -s 0 /var/lib/docker/containers/*/*-json.log
```

## Configuration Files

- `docker-compose.yml` - Service definitions with resource limits
- `.env.prod` - Environment variables (NEVER commit to git)
- `.env.example` - Template for environment variables
- `nginx/nginx.conf` - Main nginx configuration (production optimized)
- `nginx/gate149.ru.conf` - Site-specific nginx configuration
- `kratos/kratos.yml` - Kratos configuration (production mode)
- `init-db.sql` - Database initialization script

## Environment Variables

See `.env.example` for all available environment variables. Key variables:

- `ENV=production`
- `NODE_ENV=production`
- `POSTGRES_DSN` - PostgreSQL connection string
- `REDIS_URL` - Redis connection string (DB 2)
- `NEXT_PUBLIC_APP_URL=https://gate149.ru`
- `WEBSOCKET_URL=wss://gate149.ru/ws`

## Network

All services run in the `gate-network-prod` bridge network, isolated from other environments.

## Volumes

Persistent data is stored in named volumes:
- `gate-postgres-data-prod` - PostgreSQL data (BACKUP REGULARLY!)
- `gate-redis-data-prod` - Redis data (AOF persistence enabled)
- `gate-nats-data-prod` - NATS data
- `gate-certbot-conf-prod` - SSL certificates
- `gate-certbot-www-prod` - Certbot webroot

## Backup Strategy

**CRITICAL:** Schedule regular backups!

### Automated Backup Script
Create a backup script (`backup.sh`):
```bash
#!/bin/bash
BACKUP_DIR="/backups/gate-prod"
DATE=$(date +%Y%m%d_%H%M%S)

# Backup databases
docker-compose exec -T postgres pg_dump -U postgres prod_app > "$BACKUP_DIR/prod_app_$DATE.sql"
docker-compose exec -T postgres pg_dump -U postgres prod_kratos > "$BACKUP_DIR/prod_kratos_$DATE.sql"

# Backup volumes
docker run --rm -v gate-postgres-data-prod:/data -v "$BACKUP_DIR:/backup" alpine tar czf "/backup/postgres_volume_$DATE.tar.gz" -C /data .
docker run --rm -v gate-redis-data-prod:/data -v "$BACKUP_DIR:/backup" alpine tar czf "/backup/redis_volume_$DATE.tar.gz" -C /data .

# Delete backups older than 30 days
find "$BACKUP_DIR" -name "*.sql" -mtime +30 -delete
find "$BACKUP_DIR" -name "*.tar.gz" -mtime +30 -delete
```

Schedule with cron:
```bash
# Run daily at 3 AM
0 3 * * * /path/to/backup.sh
```

## Notes

- This is a **production** environment - handle with care!
- All services use production-grade settings
- Debug logging is **disabled** in Kratos
- Rate limiting protects against abuse
- Resource limits prevent resource exhaustion
- Regular backups are **essential**
- Monitor disk space and logs regularly
- Review security checklist periodically
