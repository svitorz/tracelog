# TraceLog

A local technical event monitoring system that helps developers understand error patterns and system behavior.

## Project Overview

TraceLog is a lightweight Go application designed to receive, store, and analyze technical events from your applications. It provides an HTTP API for event ingestion and a CLI for querying and analyzing event data. Built with simplicity in mind, it runs entirely on your local machine with minimal setup.

## Motivation

During development and debugging, developers often need to:
- Track errors and warnings across multiple applications
- Understand when and how often specific issues occur
- Identify patterns in system behavior
- Correlate events across different environments

TraceLog provides a centralized, local solution for these needs without requiring complex infrastructure or cloud services. It's designed for developers who want quick insights into their applications' behavior during development and testing phases.

## Features 

- **Event Ingestion**: HTTP API to receive technical events from applications
- **Event Storage**: PostgreSQL-backed persistence with full event history
- **Event Querying**: CLI commands to list and filter events by severity, date range, and application
- **Error Frequency Analysis**: Built-in metrics to track error patterns over time
- **Multi-Application Support**: Track events from multiple applications and environments
- **Hexagonal Architecture**: Clean separation between business logic and infrastructure

## Installation

### Prerequisites

- Go 1.25+
- Docker and Docker Compose (for PostgreSQL)
- Git

### Steps

1. Clone the repository:

   ```bash
   git clone <repository-url>
   cd tracelog
   ```

2. Start the PostgreSQL database:

   ```bash
   docker compose up -d
   ```

3. Run database migrations:

   ```bash
   ./scripts/run-migrations.sh
   ```

4. Initialize the local instance:

   ```bash
   go run . init
   ```

5. Build the application:

   ```bash
   go build -o bin/tracelog
   ```

## Quick Start

1. **Create the database structure**:

   ```bash
   ./scripts/run-migrations.sh
   ```

2. **Configure the local instance**:

   ```bash
   go run . init
   ```

   The command asks for:
   - default monitored application
   - default environment
   - default event source

   At the end, it prints the IDs created or updated in the database.

3. **Start the HTTP server**:

   ```bash
   go run . serve
   ```

   The server will start on `http://localhost:8080` (configurable via `.env` file).

4. **Send a test event**:

   ```bash
   curl -X POST http://localhost:8080/events \
     -H "Content-Type: application/json" \
     -d '{
       "message": "Database connection timeout",
       "occurred_at": "2026-03-09T14:30:00Z",
       "application_id": "<application-id-from-init>",
       "source_id": "<source-id-from-init>",
       "environment_id": "<environment-id-from-init>",
       "payload": {
         "retry_count": 3,
         "timeout_ms": 5000
       }
     }'
   ```

   TraceLog defines the persisted severity internally based on the event data.

5. **Query events via CLI**:

   ```bash
   go run . events list --severity error
   ```

6. **View error frequency metrics**:

   ```bash
   go run . metrics errors
   ```

## Example Event

A complete event payload includes:

```json
{
  "message": "User authentication failed",
  "occurred_at": "2026-03-09T14:30:00Z",
  "application_id": "<application-id>",
  "source_id": "<source-id>",
  "environment_id": "<environment-id>",
  "payload": {
    "user_id": "12345",
    "ip_address": "192.168.1.1",
    "endpoint": "/api/login"
  }
}
```

**Persisted severity levels**: `info`, `warn`, `error`, `fatal`

TraceLog does not trust a client-provided `severity` field on HTTP ingestion. The application classifies severity internally from the message, category, and payload signals.

## CLI Usage

### Server Management

```bash
# Configure the minimum local setup
go run . init

# Start HTTP server (default port 8080)
go run . serve

# Start on custom port
go run . serve --http-port 8081
```

### Event Queries

```bash
# List all events
go run . events list

# Filter by severity
go run . events list --severity error

# Filter by date range
go run . events list --from 2026-03-01 --to 2026-03-09

# Combine filters
go run . events list --severity critical --from 2026-03-01
```

### Metrics and Analysis

```bash
# Show error frequency by day
go run . metrics errors

# Error frequency for specific date range
go run . metrics errors --from 2026-03-01 --to 2026-03-09

# Error frequency for specific application
go run . metrics errors --application-id <uuid>
```

### Database Management

```bash
# List environments (local, dev, staging, prod)
go run . db list environments

# List registered applications
go run . db list applications

# List event sources
go run . db list sources

# Optional generic seed command
go run . seed
```

`go run . seed` is currently a no-op for the MVP. Real local instance configuration must be done with `go run . init`.

## Roadmap

Future enhancements planned:

- **RabbitMQ Integration**: Async event ingestion via message queues
- **Grafana Integration**: Visual dashboards for metrics and trends
- **Event Correlation**: Link related events across applications
- **Alerting**: Configurable alerts based on error thresholds
- **Event Retention Policies**: Automatic cleanup of old events
- **Advanced Filtering**: Full-text search and complex queries
- **Export Capabilities**: Export event data in various formats

## License

MIT License

---

**Note**: This project is currently in active development. The feature set represents the MVP state. Contributions and feedback are welcome.
