# Bootstrap e Dados Iniciais - TraceLog MVP

## Visão Geral

O TraceLog agora inclui dados iniciais automáticos para facilitar o uso local e reduzir atrito no desenvolvimento.

## O que é criado automaticamente

### Durante as migrations (`./scripts/run-migrations.sh`)

A migration `000002_bootstrap_data.sql` popula:

**Environments (4):**
- `local` - Local development environment
- `dev` - Shared development environment
- `staging` - Pre-production environment
- `prod` - Production environment

**Monitored Applications (3):**
- `Sample Web API` (service_key: `sample-web-api`)
- `Background Worker` (service_key: `background-worker`)
- `Frontend App` (service_key: `frontend-app`)

**Event Sources (4):**
- `HTTP API` (type: `http`)
- `CLI Tool` (type: `cli`)
- `File Import` (type: `file`)
- `System Monitor` (type: `system`)

## Como usar

### Método 1: Via migrations (recomendado)

```bash
# 1. Inicie o PostgreSQL
docker compose up -d

# 2. Execute migrations (inclui bootstrap automático)
./scripts/run-migrations.sh

# 3. Verifique os dados criados
go run . db list environments
go run . db list applications
go run . db list sources
```

### Método 2: Via comando CLI

```bash
# Execute o comando seed manualmente (idempotente)
go run . seed
```

O comando `seed` é **idempotente** - pode ser executado múltiplas vezes sem duplicar dados.

## Comandos de inspeção

```bash
# Listar ambientes
go run . db list environments

# Listar aplicações monitoradas
go run . db list applications

# Listar fontes de eventos
go run . db list sources
```

## Exemplo prático

### 1. Obter UUIDs necessários

```bash
# Liste os ambientes
go run . db list environments
# Copie o UUID do ambiente 'local'

# Liste as aplicações
go run . db list applications
# Copie o UUID de 'Sample Web API'

# Liste as fontes
go run . db list sources
# Copie o UUID de 'HTTP API'
```

### 2. Registrar um evento

```bash
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{
    "message": "User login successful",
    "severity": "info",
    "occurred_at": "2026-03-09T12:00:00Z",
    "ingestion_channel": "http",
    "application_id": "<uuid-da-aplicacao>",
    "source_id": "<uuid-da-fonte>",
    "environment_id": "<uuid-do-ambiente>"
  }'
```

### 3. Demo completo automatizado

```bash
# Execute o script de demonstração
./scripts/demo.sh
```

Este script:
- Lista dados bootstrap
- Inicia servidor HTTP
- Registra eventos de teste (INFO e ERROR)
- Consulta eventos
- Mostra frequência de erros
- Limpa recursos

## Arquitetura

```
migrations/
├── 000001_initial_schema.sql      # Schema + environments
└── 000002_bootstrap_data.sql      # Applications + sources

cmd/
├── seed.go                        # Comando `tracelog seed`
└── db.go                          # Comandos `tracelog db list *`
```

## Notas importantes

- **Idempotência**: Todos os INSERTs usam `ON CONFLICT DO NOTHING`
- **UUIDs**: Gerados automaticamente pelo PostgreSQL
- **Opcional**: O bootstrap não impede uso com dados customizados
- **Reset**: Para limpar e recriar: `docker compose down -v && docker compose up -d`

## Próximos passos

Após o bootstrap, você pode:

1. **Iniciar o servidor**: `go run . serve`
2. **Registrar eventos**: Via HTTP API ou CLI (futuro)
3. **Consultar eventos**: `go run . events list`
4. **Ver métricas**: `go run . metrics errors`
