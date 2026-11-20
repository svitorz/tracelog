# Getting Started

This guide documents the real first-run workflow for the current TraceLog MVP.

TraceLog requires a PostgreSQL database and a minimum local setup before the HTTP server can accept events. You must run `tracelog init` before `tracelog serve`.

## Prerequisites

- Go 1.25+
- Docker and Docker Compose
- a local shell environment capable of running the project scripts

## 1. Start PostgreSQL

Start the bundled PostgreSQL service:

```bash
docker compose up -d postgres
```

You can verify the container state with:

```bash
docker compose ps
```

## 2. Configure Environment Variables

Create `.env` from the example file if it does not exist:

```bash
cp .env.example .env
```

Review the values used by the application, especially:

- `HTTP_PORT`
- `DB_HOST`
- `DB_PORT`
- `DB_NAME`
- `DB_USER`
- `DB_PASSWORD`
- `DB_SSLMODE`

The CLI and HTTP server read configuration from `.env` at the project root.

## 3. Run Database Migrations

Apply the SQL schema:

```bash
go run . migrate
```

What this command does:

- reads the database settings from `.env`
- applies, in order, the migrations embedded in the binary that are not yet recorded in `schema_migrations`
- does nothing when the database is already up to date

PostgreSQL must already be running (step 1).

If migrations do not run successfully, do not continue to `init` or `serve`.

## 4. Run the Initial System Setup

Initialize the minimum required data:

```bash
go run . init
```

The command is interactive. It creates or updates the minimum setup required by the MVP:

- one monitored application
- one environment
- one event source

Example session:

```text
[init] Configure the minimum local setup. Press Enter to accept defaults.

Application name [Local Application]:
Application service key [local-app]:
Application description [Default monitored application for the local MVP]:
Environment code [local]:
Environment name [Local]:
Environment description [Local development environment]:
Event source name [HTTP API]:
Event source type [http]:
Event source description [Events received via HTTP REST API]:
```

At the end, the command prints the persisted IDs:

```text
[init] Basic system setup saved successfully.
[init] Application: Local Application (local-app) [<application-id>]
[init] Environment: Local (local) [<environment-id>]
[init] Event source: HTTP API (http) [<source-id>]
[init] The application is ready to start with `tracelog serve`.
```

Keep these IDs. They are useful for manual event submission and CLI filtering.

## 5. Start the HTTP Server

Start the local server:

```bash
go run . serve
```

Expected startup flow:

```text
[serve] tracelog (local)
[serve] Initializing PostgreSQL connection...
[serve] PostgreSQL connected at localhost:5432/tracelog
[serve] Initializing application dependencies...
[serve] HTTP server listening on port 8080
[serve] Ready to receive events at POST /events
```

Important:

- `serve` checks whether the basic setup already exists
- if the setup is missing, startup fails
- the documented fix is to run `go run . init` first

## 6. Send an Example HTTP Event

With the server running, submit an event:

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

Expected response:

```json
{
  "id": "<event-id>",
  "external_id": "",
  "received_at": "2026-03-09T14:30:01Z",
  "created_at": "2026-03-09T14:30:01Z"
}
```

Current MVP behavior that matters during ingestion:

- `severity` is derived internally and is not taken from the request as a trusted field
- incomplete or partially malformed input may still be normalized and stored
- normalization details may be written into the stored payload
- HTTP errors are reserved for cases where the application cannot persist any event record

## 7. Query Data Through the CLI

List recent events:

```bash
go run . events list --limit 10
```

Filter by severity:

```bash
go run . events list --severity error
```

Filter by application:

```bash
go run . events list --application-id <application-id-from-init>
```

Show error frequency:

```bash
go run . metrics errors
```

Inspect the bootstrap data:

```bash
go run . db list applications
go run . db list environments
go run . db list sources
```

## First-Run Checklist

Use this order for every fresh local setup:

1. Start PostgreSQL
2. Create and review `.env`
3. Run migrations
4. Run `go run . init`
5. Run `go run . serve`
6. Send a test event to `POST /events`
7. Query stored data with `go run . events list`

## Troubleshooting

### `serve` fails before opening the HTTP port

Most common causes in the current MVP:

- PostgreSQL is not running
- `.env` points to the wrong database
- migrations have not been applied
- `go run . init` has not been executed yet

### Event ingestion returns an HTTP error

In the current MVP, that usually means TraceLog could not persist any event record. Check:

- database connectivity
- migrated schema availability
- whether the basic setup exists
- whether the referenced IDs exist when you are not relying on the initialized defaults
