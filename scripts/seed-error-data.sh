#!/bin/bash
set -euo pipefail

# Script para popular dados de teste de erros no TraceLog

API_URL="http://localhost:8080/events"

# Pega UUIDs de ambiente e aplicação do banco (assumindo dados seed da migration)
echo "[seed] Consultando UUIDs de ambiente..."
set -a && source .env && set +a

# Para este exemplo, vamos usar UUIDs fixos que devem existir após as migrations
ENV_LOCAL="11111111-1111-1111-1111-111111111111"  # ambiente local
APP_SAMPLE="22222222-2222-2222-2222-222222222222" # aplicação sample
SOURCE_API="33333333-3333-3333-3333-333333333333" # source API

echo "[seed] Criando eventos de erro para demonstração..."

# Erros no dia 2026-03-01
for i in {1..5}; do
  curl -s -X POST "$API_URL" \
    -H "Content-Type: application/json" \
    -d "{
      \"message\": \"Database connection timeout - attempt $i\",
      \"severity\": \"error\",
      \"occurred_at\": \"2026-03-01T1${i}:00:00Z\",
      \"ingestion_channel\": \"http\",
      \"application_id\": \"$APP_SAMPLE\",
      \"source_id\": \"$SOURCE_API\",
      \"environment_id\": \"$ENV_LOCAL\"
    }" > /dev/null
done

# Erros no dia 2026-03-05
for i in {1..12}; do
  curl -s -X POST "$API_URL" \
    -H "Content-Type: application/json" \
    -d "{
      \"message\": \"Payment gateway error - retry $i\",
      \"severity\": \"error\",
      \"occurred_at\": \"2026-03-05T$(printf '%02d' $((10 + i % 12))):30:00Z\",
      \"ingestion_channel\": \"http\",
      \"application_id\": \"$APP_SAMPLE\",
      \"source_id\": \"$SOURCE_API\",
      \"environment_id\": \"$ENV_LOCAL\"
    }" > /dev/null
done

# Erros no dia 2026-03-08
for i in {1..8}; do
  curl -s -X POST "$API_URL" \
    -H "Content-Type: application/json" \
    -d "{
      \"message\": \"External API timeout - service unavailable\",
      \"severity\": \"error\",
      \"occurred_at\": \"2026-03-08T0${i}:15:00Z\",
      \"ingestion_channel\": \"http\",
      \"application_id\": \"$APP_SAMPLE\",
      \"source_id\": \"$SOURCE_API\",
      \"environment_id\": \"$ENV_LOCAL\"
    }" > /dev/null
done

# Erros no dia 2026-03-09
for i in {1..3}; do
  curl -s -X POST "$API_URL" \
    -H "Content-Type: application/json" \
    -d "{
      \"message\": \"Authentication failed - invalid token\",
      \"severity\": \"error\",
      \"occurred_at\": \"2026-03-09T0${i}:00:00Z\",
      \"ingestion_channel\": \"http\",
      \"application_id\": \"$APP_SAMPLE\",
      \"source_id\": \"$SOURCE_API\",
      \"environment_id\": \"$ENV_LOCAL\"
    }" > /dev/null
done

# Alguns eventos INFO para não ter só erros
for i in {1..10}; do
  curl -s -X POST "$API_URL" \
    -H "Content-Type: application/json" \
    -d "{
      \"message\": \"User login successful\",
      \"severity\": \"info\",
      \"occurred_at\": \"2026-03-0${i}T12:00:00Z\",
      \"ingestion_channel\": \"http\",
      \"application_id\": \"$APP_SAMPLE\",
      \"source_id\": \"$SOURCE_API\",
      \"environment_id\": \"$ENV_LOCAL\"
    }" > /dev/null
done

echo "[seed] ✓ Criados 28 eventos de erro distribuídos em 4 dias (Mar 01, 05, 08, 09)"
echo "[seed] ✓ Criados 10 eventos info para comparação"
echo ""
echo "Agora você pode testar:"
echo "  go run . metrics errors --from 2026-03-01 --to 2026-03-09"
