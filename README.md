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

## Features (MVP)

- **Event Ingestion**: HTTP API to receive technical events from applications
- **Event Storage**: PostgreSQL-backed persistence with full event history
- **Event Querying**: CLI commands to list and filter events by severity, date range, and application
- **Error Frequency Analysis**: Built-in metrics to track error patterns over time
- **Multi-Application Support**: Track events from multiple applications and environments
- **Hexagonal Architecture**: Clean separation between business logic and infrastructure

## Installation

### Prerequisites

- Go 1.21+
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

4. Build the application:

   ```bash
   go build -o bin/tracelog
   ```

## Quick Start

1. **Start the HTTP server**:

   ```bash
   go run . serve
   ```

   The server will start on `http://localhost:8080` (configurable via `.env` file).

2. **Send a test event**:

   ```bash
   curl -X POST http://localhost:8080/events \
     -H "Content-Type: application/json" \
     -d '{
       "message": "Database connection timeout",
       "severity": "error",
       "occurred_at": "2026-03-09T14:30:00Z",
       "ingestion_channel": "http"
     }'
   ```

3. **Query events via CLI**:

   ```bash
   go run . events list --severity error
   ```

4. **View error frequency metrics**:

   ```bash
   go run . metrics errors
   ```

## Example Event

A complete event payload includes:

```json
{
  "message": "User authentication failed",
  "severity": "error",
  "occurred_at": "2026-03-09T14:30:00Z",
  "ingestion_channel": "http",
  "application_id": "550e8400-e29b-41d4-a716-446655440000",
  "source_id": "660e8400-e29b-41d4-a716-446655440001",
  "environment_id": "770e8400-e29b-41d4-a716-446655440002",
  "metadata": {
    "user_id": "12345",
    "ip_address": "192.168.1.1",
    "endpoint": "/api/login"
  }
}
```

**Severity levels**: `info`, `warning`, `error`, `critical`

## CLI Usage

### Server Management

```bash
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

# Re-seed default data (idempotent)
go run . seed
```

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
