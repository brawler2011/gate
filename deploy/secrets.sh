#!/bin/sh
# Usage: sh deploy/secrets.sh kubectl --context gate -n gate
set -eu

escape() { jq -rn --arg value "$1" '$value | @uri'; }

postgres_dsn="postgres://postgres:$(escape "$POSTGRES_PASSWORD")@postgres:5432/gate?sslmode=disable"
redis_url="redis://:$(escape "$REDIS_PASSWORD")@redis:6379/0"

"$@" create secret generic gate-secrets --dry-run=client -o yaml \
  --from-literal=POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
  --from-literal=REDIS_PASSWORD="$REDIS_PASSWORD" \
  --from-literal=POSTGRES_DSN="$postgres_dsn" \
  --from-literal=REDIS_URL="$redis_url" \
  --from-literal=S3_ACCESS_KEY="$S3_ACCESS_KEY" \
  --from-literal=S3_SECRET_KEY="$S3_SECRET_KEY" \
  --from-literal=RESEND_API_KEY="$RESEND_API_KEY" \
  | "$@" apply -f -
