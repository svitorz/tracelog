# TraceLog CLI

## Overview

The TraceLog CLI provides commands to manage the HTTP server, query events, analyze metrics, and inspect database entities. All commands use configuration from the `.env` file at the project root.

**Binary Name**: `tracelog`

**Usage**: `tracelog [command] [flags]`

**Global Flags**:
- `--help, -h`: Show help for any command

---

## Command Reference

### serve

Start the HTTP server to receive technical events via POST requests.

**Usage**:
```bash
tracelog serve [--http-port PORT]
```

**Description**:

Initializes the HTTP server for event ingestion. The server listens on the configured port and provides two endpoints:
- `POST /events`: Receive technical events
- `GET /health`: Health check endpoint

The server runs in the foreground and can be stopped with `Ctrl+C` (SIGINT).

**Flags**:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--http-port` | int | `HTTP_PORT` from `.env` | Port to listen on (1-65535) |

**Examples**:

```bash
# Start with default port from .env file
tracelog serve

# Start on custom port (overrides .env)
tracelog serve --http-port 8081

# Start and verify it's running
tracelog serve &
curl http://localhost:8080/health
```

**Output**:

```
[serve] tracelog (local)
[serve] Initializing PostgreSQL connection...
[serve] PostgreSQL connected at localhost:5432/tracelog
[serve] Initializing application dependencies...
[serve] HTTP server listening on port 8080
[serve] Ready to receive events at POST /events
```

**Use Cases**:
- Run locally during development
- Accept events from applications being tested
- Serve as the event ingestion endpoint for local microservices

**Graceful Shutdown**:

Send `SIGINT` (Ctrl+C) or `SIGTERM` to stop the server gracefully:

```bash
# Running in foreground
^C  # Ctrl+C

# Running in background
kill -TERM <pid>
```

---

### events list

Query and display technical events with filtering capabilities.

**Usage**:
```bash
tracelog events list [flags]
```

**Description**:

Retrieves technical events from the database with optional filters. Results are displayed in a tabular format showing key event details. By default, returns the 50 most recent events.

**Flags**:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--severity` | string | (all) | Filter by severity: `debug`, `info`, `warn`, `error`, `fatal` |
| `--application-id` | UUID | (all) | Filter by application UUID |
| `--environment-id` | UUID | (all) | Filter by environment UUID |
| `--from` | string | (none) | Start date/time (RFC3339 or YYYY-MM-DD) |
| `--to` | string | (none) | End date/time (RFC3339 or YYYY-MM-DD) |
| `--limit` | int | `50` | Maximum number of events to return |

**Date Format Support**:
- RFC3339: `2026-03-09T14:30:00Z`
- Date only: `2026-03-09` (assumes 00:00:00 UTC)

**Examples**:

```bash
# List all recent events (last 50)
tracelog events list

# List only error events
tracelog events list --severity error

# List critical events only
tracelog events list --severity fatal

# Events from a specific application
tracelog events list --application-id 550e8400-e29b-41d4-a716-446655440000

# Events in production environment
tracelog events list --environment-id 770e8400-e29b-41d4-a716-446655440002

# Events within a date range (date only)
tracelog events list --from 2026-03-01 --to 2026-03-09

# Events within a specific time range (RFC3339)
tracelog events list --from 2026-03-09T00:00:00Z --to 2026-03-09T23:59:59Z

# Combined filters: errors in production during March
tracelog events list \
  --severity error \
  --environment-id 770e8400-e29b-41d4-a716-446655440002 \
  --from 2026-03-01 \
  --to 2026-03-31

# Get more results
tracelog events list --severity warn --limit 100

# Get fewer results for quick overview
tracelog events list --limit 10
```

**Output Format**:

```
OCCURRED_AT                MESSAGE                                  SEVERITY    APPLICATION_ID
-----------                -------                                  --------    --------------
2026-03-09T14:30:00Z      Database connection timeout              error       550e8400-e29b-...
2026-03-09T14:29:55Z      User login successful                    info        550e8400-e29b-...
2026-03-09T14:29:50Z      API request completed in 45ms            debug       660e8400-e29b-...

Total: 3 event(s)
```

**Empty Results**:

```
No events found for the provided filters.
```

**Use Cases**:
- Debug issues by finding error events
- Verify events were received from applications
- Analyze event patterns over time
- Identify problematic applications or environments
- Audit system activity

**Tips**:
- Start with broad queries, then narrow with filters
- Use `--limit 10` for quick checks
- Combine filters to focus on specific scenarios
- Use date ranges to analyze specific time periods

---

### import logs

Import technical events from one or more log files.

**Usage**:
```bash
tracelog import logs --file PATH [--file PATH...]
```

**Description**:

Reads each file line by line and tries to normalize the maximum possible amount of information from each record. The importer currently supports:

- JSON log lines
- `timestamp + level + message` log lines
- plain text fallback

Each persisted event uses `ingestion_channel=file` and preserves import metadata, including raw line content and parser errors when available.

**Flags**:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--file` | string array | required | Path to a log file to import. Repeat the flag for multiple files. |

**Examples**:

```bash
tracelog import logs --file ./app.log
tracelog import logs --file ./app.log --file ./worker.log
```

**Output**:

```
Log import summary
Files processed: 2
Lines read: 482
Persisted: 482
Normalized: 301
Partially interpreted: 143
Uncategorizable: 38
```

---

### metrics errors

Display error frequency metrics grouped by day.

**Usage**:
```bash
tracelog metrics errors [flags]
```

**Description**:

Calculates and displays the count of error-level events (severity=`error`) grouped by day. Useful for identifying error trends, spikes, and patterns over time. Provides a summary with total errors and average errors per day.

**Flags**:

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--application-id` | UUID | (all) | Filter by application UUID |
| `--environment-id` | UUID | (all) | Filter by environment UUID |
| `--from` | string | (none) | Start date/time (RFC3339 or YYYY-MM-DD) |
| `--to` | string | (none) | End date/time (RFC3339 or YYYY-MM-DD) |

**Examples**:

```bash
# Show error frequency for all applications (all time)
tracelog metrics errors

# Errors in March 2026
tracelog metrics errors --from 2026-03-01 --to 2026-03-31

# Errors for a specific application
tracelog metrics errors --application-id 550e8400-e29b-41d4-a716-446655440000

# Errors in production environment
tracelog metrics errors --environment-id 770e8400-e29b-41d4-a716-446655440002

# Errors for specific app in production during last week
tracelog metrics errors \
  --application-id 550e8400-e29b-41d4-a716-446655440000 \
  --environment-id 770e8400-e29b-41d4-a716-446655440002 \
  --from 2026-03-01 \
  --to 2026-03-09

# Compare error rates week-over-week
tracelog metrics errors --from 2026-03-01 --to 2026-03-07
tracelog metrics errors --from 2026-03-08 --to 2026-03-14
```

**Output Format**:

```
DATE        ERROR_COUNT
----        -----------
2026-03-01  12
2026-03-02  8
2026-03-03  15
2026-03-04  3
2026-03-05  7
2026-03-06  10
2026-03-07  5
2026-03-08  18
2026-03-09  6

Summary:
  Total days: 9
  Total errors: 84
  Average errors/day: 9.3
```

**Empty Results**:

```
No errors found for the provided filters.
```

**Interpreting Results**:

- **Spikes**: Days with significantly higher counts indicate incidents or issues
- **Trends**: Increasing counts over time suggest degrading system health
- **Baseline**: Consistent low counts represent normal operational errors
- **Zero days**: Days without errors (not shown in output)

**Use Cases**:
- Monitor application health over time
- Identify error spikes and correlate with deployments
- Compare error rates between environments
- Set baseline expectations for error frequency
- Detect degrading system performance early

**Advanced Analysis**:

```bash
# Compare production vs staging error rates
tracelog metrics errors --environment-id <prod-id> --from 2026-03-01 --to 2026-03-09
tracelog metrics errors --environment-id <staging-id> --from 2026-03-01 --to 2026-03-09

# Track errors before and after a deployment
tracelog metrics errors --from 2026-03-08 --to 2026-03-08  # Before
tracelog metrics errors --from 2026-03-09 --to 2026-03-09  # After

# Monthly error report for all applications
tracelog metrics errors --from 2026-03-01 --to 2026-03-31
```

---

### db list

Inspect database entities (environments, applications, event sources).

**Usage**:
```bash
tracelog db list [entity]
```

**Available Entities**:
- `environments`: List all deployment environments
- `applications`: List all monitored applications
- `sources`: List all event sources

**Description**:

Displays reference data stored in the database. Use these commands to find UUIDs needed for filtering events and metrics.

#### db list environments

List all deployment environments.

**Usage**:
```bash
tracelog db list environments
```

**Output**:

```
ID                                    CODE      NAME          DESCRIPTION
--                                    ----      ----          -----------
550e8400-e29b-41d4-a716-446655440000  local     Local         Local development environment
660e8400-e29b-41d4-a716-446655440001  dev       Development   Shared development environment
770e8400-e29b-41d4-a716-446655440002  staging   Staging       Pre-production environment
880e8400-e29b-41d4-a716-446655440003  prod      Production    Production environment

Total: 4 environment(s)
```

**Use Cases**:
- Find environment UUIDs for filtering
- Verify available environments
- Reference environment codes

#### db list applications

List all monitored applications.

**Usage**:
```bash
tracelog db list applications
```

**Output**:

```
ID                                    NAME                      SERVICE_KEY         DESCRIPTION
--                                    ----                      -----------         -----------
550e8400-e29b-41d4-a716-446655440010  User Auth API             user-auth-api       Authentication service
660e8400-e29b-41d4-a716-446655440011  Order Processing Service  order-processor     Order fulfillment worker
770e8400-e29b-41d4-a716-446655440012  Payment Gateway           payment-gateway     Payment processing

Total: 3 application(s)
```

**Use Cases**:
- Find application UUIDs for filtering
- See all registered applications
- Verify application service keys

#### db list sources

List all event sources.

**Usage**:
```bash
tracelog db list sources
```

**Output**:

```
ID                                    NAME              SOURCE_TYPE    DESCRIPTION
--                                    ----              -----------    -----------
550e8400-e29b-41d4-a716-446655440020  REST API          api            HTTP API endpoints
660e8400-e29b-41d4-a716-446655440021  Background Worker worker         Async job processor
770e8400-e29b-41d4-a716-446655440022  Scheduled Tasks   scheduler      Cron jobs and timers
880e8400-e29b-41d4-a716-446655440023  Message Consumer  consumer       Queue message processor

Total: 4 source(s)
```

**Use Cases**:
- Find source UUIDs for event registration
- Understand available source types
- Reference source classifications

**Examples**:

```bash
# View all environments and copy production UUID
tracelog db list environments

# View all applications to find specific app UUID
tracelog db list applications

# View all sources to understand event origins
tracelog db list sources

# Chain commands: list apps, then filter events by app
APP_ID=$(tracelog db list applications | grep "User Auth API" | awk '{print $1}')
tracelog events list --application-id $APP_ID --severity error
```

---

### seed

Bootstrap the database with default reference data.

**Usage**:
```bash
tracelog seed
```

**Description**:

Populates the database with default entities required for basic operation. This command is **idempotent** and can be run multiple times safely. It uses `ON CONFLICT DO NOTHING` to avoid duplicates.

**What Gets Seeded**:

1. **Environments**:
   - `local`: Local development environment
   - `dev`: Shared development environment
   - `staging`: Pre-production environment
   - `prod`: Production environment

2. **Monitored Applications** (examples):
   - Sample applications for testing
   - Can be customized via migration or direct SQL

3. **Event Sources** (examples):
   - `api`: REST API endpoints
   - `worker`: Background job processors
   - `scheduler`: Scheduled tasks
   - `consumer`: Message queue consumers

**Examples**:

```bash
# First-time database setup
./scripts/run-migrations.sh
tracelog seed

# Re-seed after database changes (safe)
tracelog seed

# Verify seeded data
tracelog db list environments
tracelog db list applications
tracelog db list sources
```

**Output**:

```
[seed] Connecting to PostgreSQL...
[seed] Seeding default data...
[seed] ✓ Environments: 4 inserted
[seed] ✓ Applications: 3 inserted
[seed] ✓ Event sources: 4 inserted
[seed] Bootstrap complete!
```

**Use Cases**:
- Initial project setup
- Reset reference data to defaults
- Add missing default entities after manual deletions
- Setup test databases

**Note**: This command does NOT seed technical events, only reference data.

---

## Common Workflows

### Initial Setup

```bash
# 1. Start database
docker compose up -d

# 2. Run migrations
./scripts/run-migrations.sh

# 3. Seed reference data
tracelog seed

# 4. Verify setup
tracelog db list environments
tracelog db list applications
```

### Development Workflow

```bash
# 1. Start the server
tracelog serve

# 2. In another terminal, send test events
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{ ... }'

# 3. Query events
tracelog events list --severity error

# 4. Check error frequency
tracelog metrics errors
```

### Debugging Issues

```bash
# 1. Find recent errors
tracelog events list --severity error --limit 20

# 2. Get specific application errors
tracelog db list applications  # Get app UUID
tracelog events list --application-id <UUID> --severity error

# 3. Analyze error trends
tracelog metrics errors --application-id <UUID> --from 2026-03-01
```

### Environment Comparison

```bash
# Get environment UUIDs
tracelog db list environments

# Compare error counts
tracelog metrics errors --environment-id <staging-UUID> --from 2026-03-01
tracelog metrics errors --environment-id <prod-UUID> --from 2026-03-01
```

---

## Configuration

All CLI commands use configuration from `.env`:

```env
APP_NAME=tracelog
APP_ENV=local
HTTP_PORT=8080
DB_HOST=localhost
DB_PORT=5432
DB_NAME=tracelog
DB_USER=postgres
DB_PASSWORD=postgres
DB_SSLMODE=disable
```

**Override Priority**:
1. Command-line flags (highest)
2. Environment variables
3. `.env` file (lowest)

---

## Output Formats

### Tables

Tabular output uses aligned columns with headers:

```
COLUMN_1    COLUMN_2    COLUMN_3
--------    --------    --------
value1      value2      value3
```

### Summary

Commands provide summary information after main output:

```
Total: X item(s)
Average: Y per day
```

---

## Error Handling

### Database Connection Errors

```
Error: failed to open PostgreSQL connection: connection refused
```

**Solution**: Ensure PostgreSQL is running (`docker compose up -d`)

### Invalid UUID

```
Error: invalid application_id: must be a valid UUID
```

**Solution**: Use `tracelog db list applications` to find correct UUIDs

### Invalid Date Format

```
Error: invalid --from value: expected RFC3339 or YYYY-MM-DD
```

**Solution**: Use `2026-03-09` or `2026-03-09T14:30:00Z`

### No Results

```
No events found for the provided filters.
```

**Not an error**: No data matches your filters. Try broader criteria.

---

## Tips and Best Practices

1. **Use date shortcuts**: `--from 2026-03-01` is easier than full RFC3339
2. **Start broad, then narrow**: Query without filters first, then add specificity
3. **Save UUIDs**: Export common UUIDs to shell variables for reuse
4. **Limit results**: Use `--limit` to avoid overwhelming output
5. **Check reference data**: Use `db list` commands to find valid UUIDs
6. **Monitor trends**: Run `metrics errors` regularly to track application health
7. **Filter by environment**: Production and staging should be analyzed separately

---

**Last Updated**: March 2026  
**CLI Version**: 1.0 (MVP)
