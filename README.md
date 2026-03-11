# TraceLog

TraceLog is a local technical event monitoring MVP built in Go with PostgreSQL. It receives events over HTTP, stores them in a relational database, and exposes CLI commands to inspect stored events and basic error metrics.

## Project Scope

The current MVP supports:

- HTTP ingestion through `POST /events`
- file ingestion through `tracelog import logs --file ...`
- PostgreSQL-backed event persistence
- Interactive local bootstrap through `tracelog init`
- Event listing through the CLI
- Error frequency reporting through the CLI

The current MVP does not provide dashboards, background consumers, alerting, or automated environment provisioning.

## Prerequisites

- Go 1.25+
- Docker and Docker Compose
- Git

## Getting Started

TraceLog must be initialized before it can receive events. The `serve` command checks for the minimum system setup and fails if `tracelog init` has not been executed successfully.

For a complete step-by-step guide, see [docs/getting-started.md](/home/svitorz/dev/tracelog/docs/getting-started.md).

### 1. Clone the repository

```bash
git clone <repository-url>
cd tracelog
```

### 2. Start PostgreSQL

```bash
docker compose up -d postgres
```

### 3. Configure environment variables

Create a local `.env` file if it does not exist:

```bash
cp .env.example .env
```

Review the database and HTTP settings in `.env` before continuing.

### 4. Run migrations

```bash
./scripts/run-migrations.sh
```

This script applies the SQL files in `migrations/` and starts the `postgres` container if needed.

### 5. Initialize the local MVP

```bash
go run . init
```

`init` is required. It interactively creates or updates:

- one monitored application
- one environment
- one event source

At the end of the command, TraceLog prints the persisted IDs. Keep them available for manual testing and CLI queries.

### 6. Start the HTTP server

```bash
go run . serve
```

If the minimum setup is missing, `serve` stops with an error and instructs you to run `tracelog init` first.

### 7. Send an example event

```bash
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Database connection timeout",
    "occurred_at": "2026-03-09T14:30:00Z",
    "category": "system",
    "application_id": "<application-id-from-init>",
    "source_id": "<source-id-from-init>",
    "environment_id": "<environment-id-from-init>",
    "payload": {
      "retry_count": 3,
      "timeout_ms": 5000
    }
  }'
```

Notes about current ingestion behavior:

- persisted `severity` is derived internally by the application
- the API attempts to normalize incomplete or partially malformed payloads
- the API only returns an HTTP error when it cannot persist any record

### 8. Query stored events

```bash
go run . events list --limit 10
```

Example filtered query:

```bash
go run . events list --severity error
```

Example metric query:

```bash
go run . metrics errors
```

## Common Commands

### Bootstrap and server

```bash
go run . init
go run . serve
go run . serve --http-port 8081
```

### Event queries

```bash
go run . events list
go run . events list --severity error
go run . events list --application-id <uuid>
go run . events list --environment-id <uuid>
go run . events list --from 2026-03-01 --to 2026-03-09
```

### File ingestion

```bash
go run . import logs --file ./app.log
go run . import logs --file ./app.log --file ./worker.log
```

### Metrics

```bash
go run . metrics errors
go run . metrics errors --application-id <uuid>
go run . metrics errors --from 2026-03-01 --to 2026-03-09
```

### Database inspection

```bash
go run . db list environments
go run . db list applications
go run . db list sources
```

## Event Model Notes

The HTTP ingestion endpoint accepts these fields:

- `message`
- `occurred_at`
- `category`
- `payload`
- `external_id`
- `trace_id`
- `correlation_id`
- `application_id`
- `source_id`
- `environment_id`

Important current behaviors:

- `severity` is not accepted as a trusted client input on HTTP ingestion
- `category` may be normalized to `unknown`
- invalid or missing `occurred_at` may fall back to `received_at`
- normalization details may be preserved inside the stored payload
- file ingestion preserves `raw_line`, `normalization_status`, and `parse_error` when available

## Additional Documentation

- [Getting Started](/home/svitorz/dev/tracelog/docs/getting-started.md)
- [HTTP API](/home/svitorz/dev/tracelog/docs/http-api.md)
- [CLI Reference](/home/svitorz/dev/tracelog/docs/cli.md)
- [Architecture](/home/svitorz/dev/tracelog/docs/architecture.md)
- [Development Notes](/home/svitorz/dev/tracelog/docs/development.md)
- [Domain Notes](/home/svitorz/dev/tracelog/docs/domain.md)

## License

MIT License
