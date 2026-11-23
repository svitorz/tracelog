#!/usr/bin/env bash
# Demonstração de ponta a ponta (spec 0004, seção 4.7). Requer só Docker e curl.
#
#   cp .env.example .env
#   ./scripts/demo.sh        # ou: make demo
#
# Sobe o compose, importa examples/logs com a CLI dentro do container, envia eventos por
# HTTP e consulta o resultado. Pode rodar de novo: o import e o POST são idempotentes.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ -z "${COMPOSE_ENV_FILES:-}" && ! -f .env ]]; then
  echo "Falta o .env: rode cp .env.example .env" >&2
  exit 1
fi

show() { printf '\n$ %s\n' "$*"; }

# A CLI no container, com os exemplos montados só para leitura.
tracelog() {
  docker compose --progress quiet run --rm --no-deps -T -v "$PWD/examples/logs:/examples:ro" api "$@"
}

post_event() {
  curl -s -w '\n[HTTP %{http_code}]\n' -X POST "$api/events" "$@"
}

show "docker compose up -d --wait"
# --progress quiet esconde só o progresso; erros continuam aparecendo.
docker compose --progress quiet build
docker compose --progress quiet up -d --wait
docker compose ps -a --format 'table {{.Service}}\t{{.Status}}'

api="http://$(docker compose port api 8080)"

show "tracelog import logs --file /examples/checkout-api.log --file /examples/auth-service.log --file /examples/legacy-gateway.log"
tracelog import logs \
  --file /examples/checkout-api.log \
  --file /examples/auth-service.log \
  --file /examples/legacy-gateway.log

show "curl -X POST $api/events   # evento novo"
post_event -H 'Content-Type: application/json' \
  -d '{"external_id":"demo-0001","message":"payment provider timeout","occurred_at":"2026-03-14T18:00:00Z","category":"system"}'

show "curl -X POST $api/events   # reenvio do mesmo external_id"
post_event -H 'Content-Type: application/json' \
  -d '{"external_id":"demo-0001","message":"payment provider timeout","occurred_at":"2026-03-14T18:00:00Z","category":"system"}'

show "curl -X POST $api/events   # Content-Type que não é JSON"
post_event -H 'Content-Type: text/plain' -d 'not json'

show "tracelog events list --severity fatal --from 2026-03-01 --to 2026-03-14"
tracelog events list --severity fatal --from 2026-03-01 --to 2026-03-14

show "tracelog metrics errors --from 2026-03-01 --to 2026-03-14"
tracelog metrics errors --from 2026-03-01 --to 2026-03-14
