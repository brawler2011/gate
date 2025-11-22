# tester

[![Go](https://img.shields.io/badge/Go-00ADD8?style=flat-square&logo=go&logoColor=white)](https://golang.org/)
[![Fiber](https://img.shields.io/badge/Fiber-008080?style=flat-square&logo=go&logoColor=white)](https://gofiber.io/)
[![Docker](https://img.shields.io/badge/Docker-2496ED?style=flat-square&logo=docker&logoColor=white)](https://www.docker.com/)
[![Valkey](https://img.shields.io/badge/Valkey-A00?style=flat-square&logo=redis&logoColor=white)](https://valkey.io/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-336791?style=flat-square&logo=postgresql&logoColor=white)](https://www.postgresql.org/)
[![Redis](https://img.shields.io/badge/Redis-B00?style=flat-square&logo=redis&logoColor=white)](https://redis.io/)
[![Pandoc](https://img.shields.io/badge/Pandoc-4A4A4A?style=flat-square)](https://pandoc.org/)
[![OpenAPI v3](https://img.shields.io/badge/OpenAPI-v3-6BA81E?style=flat-square&logo=swagger&logoColor=white)](https://swagger.io/specification/)

`tester` is a backend service designed for managing programming competitions. It handles problems, contests,
participants, and their submissions, as well as user authentication and management. The service is developed in Go using
the Fiber framework. PostgreSQL serves as the relational database, Valkey (or Redis) is used for caching and session
management. Pandoc is used to convert problem statements from LaTeX to HTML.

For understanding the architecture, see the [documentation](https://github.com/gate149/docs).

## Features

- Manage programming contests, problems, and participant submissions.
- User authentication and management via JWT.
- LaTeX to HTML conversion for problem statements using Pandoc.
- RESTful API defined with OpenAPI.
- Websocket support for real-time updates.

## Prerequisites

Before you begin, ensure you have the following dependencies installed:

- **Docker** and **Docker Compose**: To run PostgreSQL, Pandoc, and Valkey.
- **Goose**: For applying database migrations (`go install github.com/pressly/goose/v3/cmd/goose@latest`).
- **oapi-codegen**: For generating OpenAPI code (`go install github.com/deepmap/oapi-codegen/cmd/oapi-codegen@latest`).

## 1. Running Dependencies

The service depends on PostgreSQL, Pandoc, and Valkey, which can be run using Docker Compose. Below is an
example `docker-compose.yml` configuration:

```yaml
version: '3.8'
services:
  pandoc:
    image: pandoc/latex
    ports:
      - "4000:3030"
    command: "server"
  postgres:
    image: postgres:14.1-alpine
    restart: always
    environment:
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: supersecretpassword
      POSTGRES_DB: tester
    ports:
      - '5432:5432'
    volumes:
      - ./postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: pg_isready -U postgres -d tester
      interval: 10s
      timeout: 3s
      retries: 5
  valkey:
    image: valkey/valkey:latest
    volumes:
      - ./conf/valkey.conf:/usr/local/etc/valkey/valkey.conf
      - ./valkey-data:/data
    command: [ "valkey-server", "/usr/local/etc/valkey/valkey.conf" ]
    healthcheck:
      test: [ "CMD-SHELL", "valkey-cli ping | grep PONG" ]
      interval: 10s
      timeout: 3s
      retries: 5
    ports:
      - "6379:6379"
  nats:
    image: nats:2.10
    ports:
      - "4222:4222"
      - "8222:8222"
    volumes:
      - nats-data:/data
      - ./nats.conf:/etc/nats/nats.conf
    command: ["-c", "/etc/nats/nats.conf"]

volumes:
  postgres-data:
  valkey-data:
  nats-data:
    name: nats-data
```

Start the services in detached mode:

```bash
docker-compose up -d
```

#### NATS Configuration

```
port: 4222
http: 8222
logfile: "/data/nats.log"
```

## 2. Configuration

The application uses environment variables for configuration. Create a .env file in the project root with the following
variables:

```dotenv
# Environment type (development or production)
ENV=dev

# Address and port where the tester service will listen
ADDRESS=0.0.0.0:13000

# Address of the running Pandoc service
PANDOC=http://localhost:4000

# PostgreSQL connection string (Data Source Name)
POSTGRES_DSN=host=localhost port=5432 user=postgres password=supersecretpassword dbname=tester sslmode=disable

# Valkey/Redis connection string
REDIS_DSN=valkey://localhost:6379/0

# Secret key for signing and verifying JWT tokens
JWT_SECRET=secret

# Default admin credentials
ADMIN_USERNAME=admin
ADMIN_PASSWORD=admin

# Cache configuration
CACHE_DIR=C:\Users\You\gate7\tester\cache

NATS_URL=nats://localhost:4222
```

Important: Replace supersecretpassword, secret, admin, and other sensitive values with secure, unique
values for production.

## 3. Database Migrations

The project uses goose to manage the database schema.
Ensure goose is installed:

```bash
go install github.com/pressly/goose/v3/cmd/goose@latest
```

Apply migrations to the PostgreSQL database:

```bash
goose -dir ./migrations postgres "host=localhost port=5432 user=postgres password=supersecretpassword dbname=tester sslmode=disable" up
```

## 4. OpenAPI Code Generation

The API is defined using OpenAPI, and Go code for handlers and models is generated with oapi-codegen.
Run the generation command:

```bash
make gen
```

## 5. Running the Application

Start the tester service:

```bash
go run ./main.go
```

The service will be available at the address specified in the ADDRESS variable (e.g., http://localhost:13000).

## 6. Authentication and User Management

The service handles user authentication using JWT tokens, with credentials stored in PostgreSQL and sessions managed via
Valkey. Default admin credentials are set via ADMIN_USERNAME and ADMIN_PASSWORD in the .env file. Users can be managed
through API endpoints defined in the OpenAPI specification.