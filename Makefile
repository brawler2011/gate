.PHONY: help gen go-gen ts-gen gen-clean \
        local-backend-migrate local-backend-server local-backend-kratos \
        prod-backup build-all docker-prune

.DEFAULT_GOAL := help

# ---------------------------------------------------------------------------
# Code generation (delegates to contracts/Makefile)
# ---------------------------------------------------------------------------

gen:
	$(MAKE) -C contracts/

go-gen:
	$(MAKE) -C contracts/ go-gen

ts-gen:
	$(MAKE) -C contracts/ ts-gen

gen-clean:
	$(MAKE) -C contracts/ clean

# ---------------------------------------------------------------------------
# Deploy (delegates to deploy/Makefile)
# $(MAKE) -C deploy/ sets CWD to deploy/, so cd $* inside that Makefile works.
# ---------------------------------------------------------------------------

local-% dev-% prod-%:
	$(MAKE) -C deploy/ $@

# ---------------------------------------------------------------------------
# Local native backend (runs outside Docker, infra still in Docker)
# Reads env from deploy/local/.env.backend — copy from .env.backend.example
# and fill in your credentials before use.
# ---------------------------------------------------------------------------

LOCAL_BACKEND_ENV := deploy/local/.env

local-backend-migrate:
	cd backend && \
	set -a && . ../$(LOCAL_BACKEND_ENV) && set +a && \
	POSTGRES_DSN="postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@localhost:$$POSTGRES_PORT/$$POSTGRES_APP_DB?sslmode=disable" \
	go run . migrate

local-backend-server:
	cd backend && \
	set -a && . ../$(LOCAL_BACKEND_ENV) && set +a && \
	POSTGRES_DSN="postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@localhost:$$POSTGRES_PORT/$$POSTGRES_APP_DB?sslmode=disable" \
	REDIS_URL="redis://:$$REDIS_PASSWORD@localhost:$$REDIS_PORT/$$REDIS_DB_INDEX" \
	NATS_URL="nats://localhost:$$NATS_PORT" \
	KRATOS_URL="http://localhost:4433" \
	KRATOS_ADMIN_URL="http://localhost:4434" \
	GOJUDGE_GRPC_ADDR="localhost:5051" \
	ADDRESS="$$BACKEND_ADDRESS" \
	PRIVATE_ADDRESS="$$BACKEND_PRIVATE_ADDRESS" \
	go run . server

local-backend-kratos:
	cd backend && \
	set -a && . ../$(LOCAL_BACKEND_ENV) && set +a && \
	POSTGRES_DSN="postgres://$$POSTGRES_USER:$$POSTGRES_PASSWORD@localhost:$$POSTGRES_PORT/$$POSTGRES_APP_DB?sslmode=disable" \
	REDIS_URL="redis://:$$REDIS_PASSWORD@localhost:$$REDIS_PORT/$$REDIS_DB_INDEX" \
	NATS_URL="nats://localhost:$$NATS_PORT" \
	KRATOS_URL="http://localhost:4433" \
	KRATOS_ADMIN_URL="http://localhost:4434" \
	PRIVATE_ADDRESS="$$BACKEND_PRIVATE_ADDRESS" \
	go run . kratos

prod-backup:
	$(MAKE) -C deploy/ prod-backup

build-all:
	$(MAKE) -C deploy/ build-all

docker-prune:
	$(MAKE) -C deploy/ docker-prune

# ---------------------------------------------------------------------------
# Help
# ---------------------------------------------------------------------------

help:
	@echo ""
	@echo "Usage: make <target>"
	@echo ""
	@echo "Code generation:"
	@echo "  gen           Run go-gen + ts-gen"
	@echo "  go-gen        Generate Go code from OpenAPI specs"
	@echo "  ts-gen        Generate TypeScript code from OpenAPI specs"
	@echo "  gen-clean     Remove generated artifacts"
	@echo ""
	@echo "Deploy  (env = local | dev | prod):"
	@echo "  <env>-up      docker-compose up -d"
	@echo "  <env>-down    docker-compose down"
	@echo "  <env>-restart docker-compose restart"
	@echo "  <env>-logs    docker-compose logs -f"
	@echo "  <env>-ps      docker-compose ps"
	@echo "  <env>-build   docker-compose build"
	@echo "  <env>-clean   docker-compose down -v"
	@echo ""
	@echo "Local native backend (infra in Docker, backend on host):"
	@echo "  local-backend-migrate  Run DB migrations natively"
	@echo "  local-backend-server   Run API server natively (port 8080)"
	@echo "  local-backend-kratos   Run Kratos webhook server natively"
	@echo "  (reads credentials from deploy/local/.env)"
	@echo ""
	@echo "Deploy extras:"
	@echo "  <env>-ssl-init   Obtain Let's Encrypt certificate"
	@echo "  <env>-ssl-renew  Renew certificate and restart nginx"
	@echo "  prod-backup      Dump production databases"
	@echo "  build-all        Build all environments"
	@echo "  docker-prune     Remove unused Docker resources"
	@echo ""
