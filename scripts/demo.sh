#!/bin/bash
set -euo pipefail

# Demo completo do TraceLog MVP
# Este script demonstra o fluxo end-to-end do sistema

echo "======================================"
echo "TraceLog MVP - Complete Demo"
echo "======================================"
echo ""

# 1. Verificar dados bootstrap
echo "1. Listing bootstrap data..."
echo ""
echo "Environments:"
go run . db list environments
echo ""

echo "Applications:"
go run . db list applications
echo ""

echo "Event Sources:"
go run . db list sources
echo ""

# 2. Capturar UUIDs para uso
ENV_ID=$(docker compose exec -T postgres psql -U postgres -d tracelog -tAc "SELECT id FROM environment WHERE code='local' LIMIT 1")
APP_ID=$(docker compose exec -T postgres psql -U postgres -d tracelog -tAc "SELECT id FROM monitored_application WHERE service_key='sample-web-api' LIMIT 1")
SOURCE_ID=$(docker compose exec -T postgres psql -U postgres -d tracelog -tAc "SELECT id FROM event_source WHERE source_type='http' LIMIT 1")

echo "Using for demo:"
echo "  Environment: $ENV_ID (local)"
echo "  Application: $APP_ID (sample-web-api)"
echo "  Source: $SOURCE_ID (HTTP API)"
echo ""

# 3. Iniciar servidor em background
echo "2. Starting HTTP server in background..."
go run . serve > /tmp/tracelog-server.log 2>&1 &
SERVER_PID=$!
echo "   Server PID: $SERVER_PID"

# Aguardar servidor iniciar
sleep 3

# 4. Registrar eventos de teste
echo ""
echo "3. Registering test events..."

# Eventos INFO
for i in {1..3}; do
  curl -s -X POST http://localhost:8080/events \
    -H "Content-Type: application/json" \
    -d "{
      \"message\": \"User authentication successful - user_$i\",
      \"severity\": \"info\",
      \"occurred_at\": \"2026-03-09T10:0${i}:00Z\",
      \"ingestion_channel\": \"http\",
      \"application_id\": \"$APP_ID\",
      \"source_id\": \"$SOURCE_ID\",
      \"environment_id\": \"$ENV_ID\"
    }" > /dev/null
  echo "   ✓ Registered INFO event $i"
done

# Eventos ERROR
for i in {1..5}; do
  curl -s -X POST http://localhost:8080/events \
    -H "Content-Type: application/json" \
    -d "{
      \"message\": \"Database connection timeout - retry $i\",
      \"severity\": \"error\",
      \"occurred_at\": \"2026-03-09T10:1${i}:00Z\",
      \"ingestion_channel\": \"http\",
      \"application_id\": \"$APP_ID\",
      \"source_id\": \"$SOURCE_ID\",
      \"environment_id\": \"$ENV_ID\"
    }" > /dev/null
  echo "   ✓ Registered ERROR event $i"
done

echo ""
echo "4. Querying events..."
echo ""
go run . events list --limit 8

echo ""
echo "5. Checking error frequency..."
echo ""
go run . metrics errors --from 2026-03-09 --to 2026-03-09

# Cleanup
echo ""
echo "6. Cleanup..."
kill $SERVER_PID 2>/dev/null || true
echo "   ✓ Server stopped"

echo ""
echo "======================================"
echo "Demo completed successfully!"
echo "======================================"
