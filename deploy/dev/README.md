# Development Environment (dev.gate149.ru)

This is the development environment configuration for the Gate project. It runs on `dev.gate149.ru` with HTTPS.

## Prerequisites

- Docker and Docker Compose installed
- Domain `dev.gate149.ru` pointing to your server
- Ports available: 80, 443
- SSL certificates (can be obtained with certbot)

## Quick Start

1. **Copy the environment file:**
   ```bash
   cp .env.example .env.dev
   ```

2. **Update `.env.dev` with actual passwords** (if you changed them from defaults)

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
     -d dev.gate149.ru
   
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
   - Frontend: https://dev.gate149.ru
   - Backend API: https://dev.gate149.ru/api
   - Kratos Admin: https://dev.gate149.ru/kratos/admin
   - Judge0: https://dev.gate149.ru/judge0

## Services

### Core Services
- **Frontend** - Next.js application (internal port 3000)
- **Backend** - Go application (internal port 8080)
- **Nginx** - Reverse proxy with SSL (ports 80, 443)

### Authentication & Database
- **Kratos** - Identity and user management (internal ports 4433, 4434)
- **PostgreSQL** - Database server (port 5432, localhost only)
  - Database: `dev_app` - main application database
  - Database: `dev_kratos` - Kratos identity database

### Infrastructure
- **Redis** - Cache and session storage (internal, DB 1)
- **NATS** - Message broker (ports 4222, 8222, localhost only)
- **Judge0** - Code execution engine (internal)
- **Pandoc** - Document conversion (internal)

### SSL/TLS
- **Certbot** - SSL certificate management (manual, via profiles)
- **Certbot-renew** - Automatic certificate renewal (runs every 12h)

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
docker-compose logs -f nginx
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

# Access dev_app database
docker-compose exec postgres psql -U postgres -d dev_app

# Backup database
docker-compose exec postgres pg_dump -U postgres dev_app > backup.sql

# Restore database
docker-compose exec -T postgres psql -U postgres dev_app < backup.sql
```

### SSL Certificates

#### Initial Certificate
```bash
# Obtain certificate for dev.gate149.ru
docker-compose run --rm certbot certonly --webroot \
  --webroot-path=/var/www/certbot \
  --email your-email@example.com \
  --agree-tos \
  --no-eff-email \
  -d dev.gate149.ru

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

#### Check Certificate Expiry
```bash
# Check certificate expiration
docker-compose run --rm certbot certificates
```

Note: Auto-renewal runs via `certbot-renew` service every 12 hours.

### Debugging
```bash
# Enter a container shell
docker-compose exec backend sh
docker-compose exec frontend sh

# Check nginx configuration
docker-compose exec nginx nginx -t

# Reload nginx configuration
docker-compose exec nginx nginx -s reload

# Check container resource usage
docker stats

# Inspect a service
docker-compose exec backend env
```

## Deployment Workflow

1. **Build new images:**
   ```bash
   docker-compose build
   ```

2. **Pull latest code and restart:**
   ```bash
   git pull
   docker-compose build
   docker-compose up -d
   ```

3. **Check logs for errors:**
   ```bash
   docker-compose logs -f
   ```

4. **Run database migrations (if needed):**
   ```bash
   docker-compose exec backend ./core migrate
   ```

## Troubleshooting

### SSL Certificate Issues
```bash
# Check certificate files exist
docker-compose exec nginx ls -la /etc/letsencrypt/live/dev.gate149.ru/

# Test nginx configuration
docker-compose exec nginx nginx -t

# Check certbot logs
docker-compose logs certbot-renew
```

### Port Already in Use
```bash
# Check what's using ports 80/443
sudo netstat -tulpn | grep :80
sudo netstat -tulpn | grep :443

# Stop conflicting services
sudo systemctl stop apache2  # or other web server
```

### Database Connection Issues
```bash
# Check if PostgreSQL is healthy
docker-compose exec postgres pg_isready -U postgres

# Check database exists
docker-compose exec postgres psql -U postgres -c "\l"

# Recreate databases
docker-compose down
docker-compose up -d postgres
# Wait for it to be healthy
docker-compose up -d
```

### Kratos Migration Failed
```bash
# Check Kratos migrate logs
docker-compose logs kratos-migrate

# Manually run migration
docker-compose run --rm kratos-migrate
```

### Frontend Not Accessible
```bash
# Check frontend logs
docker-compose logs frontend

# Check nginx logs
docker-compose logs nginx

# Test nginx config
docker-compose exec nginx nginx -t

# Check if frontend is running
docker-compose exec frontend wget -O- http://localhost:3000
```

### Reset Everything
```bash
# WARNING: This will delete all data
docker-compose down -v
rm -rf volumes/*  # If using local volumes
docker-compose up -d
```

## Configuration Files

- `docker-compose.yml` - Service definitions
- `.env.dev` - Environment variables (not committed to git)
- `.env.example` - Template for environment variables
- `nginx/nginx.conf` - Main nginx configuration
- `nginx/dev.gate149.ru.conf` - Site-specific nginx configuration
- `kratos/kratos.yml` - Kratos configuration
- `init-db.sql` - Database initialization script

## Environment Variables

See `.env.example` for all available environment variables. Key variables:

- `ENV` - Environment name (dev)
- `NODE_ENV` - Node environment (production)
- `POSTGRES_DSN` - PostgreSQL connection string
- `REDIS_URL` - Redis connection string (DB 1)
- `KRATOS_PUBLIC_URL` - Kratos public API URL
- `JUDGE0_URL` - Judge0 API URL

## Network

All services run in the `gate-network-dev` bridge network, allowing them to communicate using service names as hostnames.

## Volumes

Persistent data is stored in named volumes:
- `gate-postgres-data-dev` - PostgreSQL data
- `gate-redis-data-dev` - Redis data (with AOF persistence)
- `gate-nats-data-dev` - NATS data
- `gate-certbot-conf-dev` - SSL certificates
- `gate-certbot-www-dev` - Certbot webroot

## Security

- **HTTPS only** - All HTTP requests redirect to HTTPS
- **HSTS enabled** - HTTP Strict Transport Security
- **Security headers** - X-Frame-Options, X-Content-Type-Options, etc.
- **Malicious request blocking** - Common attack patterns blocked
- **SSL/TLS** - Modern protocols only (TLSv1.2, TLSv1.3)

## Monitoring

### Health Checks
- Backend: `https://dev.gate149.ru/api/health`
- Frontend: Check homepage loads
- Kratos: `https://dev.gate149.ru/health/alive`
- Nginx: `https://dev.gate149.ru/health`

### Logs
```bash
# Watch all logs in real-time
docker-compose logs -f

# Filter by service
docker-compose logs -f nginx backend
```

## Notes

- This environment uses **HTTPS with SSL certificates**
- Auto-renewal of SSL certificates via certbot-renew
- Redis uses **AOF persistence** for data durability
- Suitable for **development and staging**
- Some debug logging enabled (Kratos leak_sensitive_values)
- Database port exposed only to localhost (127.0.0.1:5432)
