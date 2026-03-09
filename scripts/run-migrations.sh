#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ENV_FILE="$ROOT_DIR/.env"
MIGRATIONS_DIR="$ROOT_DIR/migrations"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "[migrations] .env not found. Copying from .env.example"
  cp "$ROOT_DIR/.env.example" "$ENV_FILE"
fi

set -a
source "$ENV_FILE"
set +a

if [[ -z "${DB_NAME:-}" || -z "${DB_USER:-}" ]]; then
  echo "[migrations] DB_NAME and DB_USER must be set in .env"
  exit 1
fi

echo "[migrations] Starting postgres service..."
docker compose -f "$ROOT_DIR/docker-compose.yml" --env-file "$ENV_FILE" up -d postgres

echo "[migrations] Applying SQL files from $MIGRATIONS_DIR"
for file in "$MIGRATIONS_DIR"/*.sql; do
  echo "[migrations] Running $(basename "$file")"
  docker compose -f "$ROOT_DIR/docker-compose.yml" --env-file "$ENV_FILE" exec -T postgres \
    psql -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$DB_NAME" -f - < "$file"
done

echo "[migrations] Done."
