#!/bin/bash
set -euo pipefail

# Script para popular dados mínimos necessários no banco

echo "[setup] Carregando configuração..."
set -a && source .env && set +a

echo "[setup] Criando aplicação e fonte de exemplo..."

# Criar aplicação sample
docker compose exec -T postgres psql -U "$DB_USER" -d "$DB_NAME" <<-SQL
  INSERT INTO monitored_application (id, name, service_key, description)
  VALUES (
    '11111111-1111-1111-1111-111111111111',
    'Sample Application',
    'sample-app',
    'Sample application for MVP demonstration'
  )
  ON CONFLICT (service_key) DO NOTHING;

  INSERT INTO event_source (id, name, source_type, description)
  VALUES (
    '22222222-2222-2222-2222-222222222222',
    'API Gateway',
    'http',
    'HTTP API Gateway source'
  )
  ON CONFLICT (name, source_type) DO NOTHING;

  -- Mostrar ambiente local (deve existir após migration)
  SELECT id, code, name FROM environment WHERE code = 'local';
SQL

echo "[setup] ✓ Aplicação e fonte criadas"
echo ""
echo "UUIDs para uso:"
echo "  Application: 11111111-1111-1111-1111-111111111111"
echo "  Source:      22222222-2222-2222-2222-222222222222"
echo "  Environment: consulte acima (code=local)"
