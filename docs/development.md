# Development Guide

This guide helps developers set up the TraceLog project locally and understand its structure for contributing.

## Prerequisites

Ensure you have the following installed:

- **Go** 1.21+ ([download](https://go.dev/dl/))
- **Docker** and **Docker Compose** ([download](https://docs.docker.com/get-docker/))
- **Git**
- **Make** (optional, for convenience)
- A terminal (bash, zsh, or similar)

## Getting Started

### 1. Clone the Repository

```bash
git clone <repository-url>
cd tracelog
```

### 2. Create Configuration File

Copy the example configuration:

```bash
cp .env.example .env
```

Edit `.env` if needed. Default values work for local development:

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

### 3. Start PostgreSQL

Use Docker Compose to start the database:

```bash
docker compose up -d
```

**Verify it's running**:

```bash
docker compose ps
```

Expected output:
```
NAME                 IMAGE                  STATUS
tracelog-postgres    postgres:16-alpine     Up (healthy)
```

**Check logs** (if needed):

```bash
docker compose logs postgres
```

### 4. Run Database Migrations

Apply the migrations embedded in the binary (PostgreSQL must be running):

```bash
go run . migrate
```

Each file in `migrations/` runs once and is recorded in `schema_migrations`. Migrations create tables only; reference data comes from `go run . init`.

**Verify migrations**:

```bash
go run . db list environments
go run . db list applications
go run . db list sources
```

### 5. Build the Application

Build the binary:

```bash
go build -o bin/tracelog
```

Or use `go run` for quick testing without building:

```bash
go run . --help
```

### 6. Run the Application

Start the HTTP server:

```bash
go run . serve
```

Expected output:
```
[serve] tracelog (local)
[serve] Initializing PostgreSQL connection...
[serve] PostgreSQL connected at localhost:5432/tracelog
[serve] Initializing application dependencies...
[serve] HTTP server listening on port 8080
[serve] Ready to receive events at POST /events
```

The server is now ready at `http://localhost:8080`.

### 7. Test the API

In another terminal, send a test event:

```bash
curl -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{
    "message": "Test event from development setup",
    "occurred_at": "2026-03-09T14:30:00Z",
    "severity": "info",
    "application_id": "550e8400-e29b-41d4-a716-446655440000",
    "source_id": "660e8400-e29b-41d4-a716-446655440001",
    "environment_id": "770e8400-e29b-41d4-a716-446655440002"
  }'
```

**Query the event**:

```bash
go run . events list --limit 1
```

---

## Running Tests

### Unit Tests

Run all tests:

```bash
go test ./...
```

Run tests with verbose output:

```bash
go test -v ./...
```

Run tests for a specific package:

```bash
go test -v ./internal/core/services/...
```

Run a specific test:

```bash
go test -v -run TestRegisterTechnicalEventUseCase ./internal/core/services/
```

### Test Coverage

Generate coverage report:

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Integration Tests

The repository integration tests run only when `TRACELOG_TEST_DATABASE_URL` is set; otherwise they are skipped. Each test creates and drops its own schema, so existing tables are not touched:

```bash
TRACELOG_TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/tracelog?sslmode=disable' \
  go test ./internal/adapters/repository/
```

---

## Project Structure

```
tracelog/
├── cmd/                          # CLI commands (Cobra)
│   ├── root.go                   # Root command and configuration
│   ├── serve.go                  # HTTP server command
│   ├── events.go                 # Event query commands
│   ├── metrics.go                # Metrics commands
│   ├── db.go                     # Database inspection commands
│   └── seed.go                   # Data seeding command
│
├── internal/                     # Private application code
│   ├── adapters/                 # External interface implementations
│   │   ├── http/                 # HTTP server and handlers
│   │   │   ├── server.go         # HTTP server setup
│   │   │   ├── event_handler.go  # Event endpoint handler
│   │   │   └── dto.go            # Data Transfer Objects
│   │   ├── repository/           # Database implementations
│   │   │   ├── postgres.go       # PostgreSQL connection
│   │   │   ├── technical_event_repository.go
│   │   │   └── error_frequency_repository.go
│   │   └── amqp/                 # RabbitMQ (future)
│   │
│   ├── core/                     # Business logic (domain layer)
│   │   ├── domain/               # Entities and value objects
│   │   │   ├── technical_event.go
│   │   │   ├── monitored_application.go
│   │   │   ├── event_source.go
│   │   │   ├── environment.go
│   │   │   ├── error_frequency.go
│   │   │   └── types.go          # Enums (Severity, Category, etc.)
│   │   ├── ports/                # Repository interfaces
│   │   │   ├── technical_event_repository.go
│   │   │   └── error_frequency_repository.go
│   │   └── services/             # Use cases
│   │       ├── register_technical_event_usecase.go
│   │       ├── list_technical_events_usecase.go
│   │       └── get_error_frequency_usecase.go
│   │
│   └── config/                   # Configuration management
│       └── config.go             # Load and parse .env
│
├── migrations/                   # Database migrations (SQL)
│   ├── 000001_initial_schema.sql
│   └── 000002_bootstrap_data.sql
│
├── scripts/                      # Utility scripts
│   ├── setup-demo-data.sh        # Populate demo data
│   └── seed-error-data.sh        # Generate sample error events
│
├── docs/                         # Documentation
│   ├── architecture.md           # System architecture
│   ├── domain.md                 # Domain model reference
│   ├── http-api.md               # API documentation
│   ├── cli.md                    # CLI commands reference
│   └── development.md            # This file
│
├── bin/                          # Compiled binaries (gitignored)
├── docker-compose.yml            # PostgreSQL container setup
├── go.mod                        # Go module dependencies
├── go.sum                        # Dependency checksums
├── main.go                       # Application entry point
├── README.md                     # Project overview
└── .env                          # Local configuration (gitignored)
```

### Architecture Layers

The project follows **Hexagonal Architecture** (Ports and Adapters):

```
┌─────────────────────────────────────────┐
│         Adapters (External)             │
│  HTTP Server │ CLI │ PostgreSQL         │
└──────────────┬──────────────────────────┘
               │
        ┌──────▼───────┐
        │    Ports     │  (Interfaces)
        └──────┬───────┘
               │
    ┌──────────▼───────────┐
    │   Use Cases          │  (Application Logic)
    │  Services Layer      │
    └──────────┬───────────┘
               │
        ┌──────▼───────┐
        │   Domain     │  (Business Rules)
        │   Entities   │
        └──────────────┘
```

**Key Principles**:
1. **Dependencies point inward**: Outer layers depend on inner layers, never the reverse
2. **Domain is independent**: Core business logic has no external dependencies
3. **Adapters are swappable**: Replace PostgreSQL with MongoDB without changing use cases
4. **Testability**: Mock interfaces for unit testing use cases

---

## Development Workflow

### Making Changes

1. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** following the architecture:
   - New entity? Add to `internal/core/domain/`
   - New use case? Add to `internal/core/services/`
   - New endpoint? Add to `internal/adapters/http/`
   - New repository method? Update `internal/core/ports/` and implement in `internal/adapters/repository/`

3. **Write tests**:
   ```bash
   # Create test file next to implementation
   touch internal/core/services/your_usecase_test.go
   ```

4. **Run tests**:
   ```bash
   go test ./...
   ```

5. **Format code**:
   ```bash
   go fmt ./...
   ```

6. **Check for issues**:
   ```bash
   go vet ./...
   ```

7. **Commit changes**:
   ```bash
   git add .
   git commit -m "feat: add your feature description"
   ```

### Adding a New Use Case

Example: Adding a "delete event" use case

1. **Define the use case** in `internal/core/services/`:

```go
// delete_technical_event_usecase.go
package services

type DeleteTechnicalEventInput struct {
    ID string
}

type DeleteTechnicalEventUseCase struct {
    repo ports.TechnicalEventRepository
}

func NewDeleteTechnicalEventUseCase(repo ports.TechnicalEventRepository) *DeleteTechnicalEventUseCase {
    return &DeleteTechnicalEventUseCase{repo: repo}
}

func (uc *DeleteTechnicalEventUseCase) Execute(ctx context.Context, input DeleteTechnicalEventInput) error {
    // Business logic here
    return uc.repo.Delete(ctx, input.ID)
}
```

2. **Add repository method** to `internal/core/ports/`:

```go
// technical_event_repository.go
type TechnicalEventRepository interface {
    Save(ctx context.Context, event *domain.TechnicalEvent) error
    FindByFilters(ctx context.Context, filters TechnicalEventFilters) ([]domain.TechnicalEvent, error)
    Delete(ctx context.Context, id string) error  // New method
}
```

3. **Implement in repository adapter** (`internal/adapters/repository/`):

```go
// technical_event_repository.go
func (r *PostgresTechnicalEventRepository) Delete(ctx context.Context, id string) error {
    _, err := r.db.ExecContext(ctx, "DELETE FROM technical_event WHERE id = $1", id)
    return err
}
```

4. **Add HTTP handler** (optional):

```go
// internal/adapters/http/event_handler.go
func (h *EventHandler) HandleDeleteEvent(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

5. **Write tests**:

```go
// delete_technical_event_usecase_test.go
func TestDeleteTechnicalEventUseCase_Execute(t *testing.T) {
    // Test implementation
}
```

### Adding a New CLI Command

Example: Adding a `tracelog events delete` command

1. **Add command to `cmd/events.go`**:

```go
var eventsDeleteCmd = &cobra.Command{
    Use:   "delete [event-id]",
    Short: "Delete a technical event",
    Args:  cobra.ExactArgs(1),
    RunE: func(cmd *cobra.Command, args []string) error {
        eventID := args[0]
        // Connect to DB, call use case
        return nil
    },
}

func init() {
    eventsCmd.AddCommand(eventsDeleteCmd)
}
```

---

## Database Management

### Direct Database Access

Connect to PostgreSQL directly:

```bash
docker compose exec postgres psql -U postgres -d tracelog
```

**Useful queries**:

```sql
-- Count events by severity
SELECT severity, COUNT(*) FROM technical_event GROUP BY severity;

-- Recent errors
SELECT occurred_at, message FROM technical_event 
WHERE severity = 'error' 
ORDER BY occurred_at DESC 
LIMIT 10;

-- Events by application
SELECT ma.name, COUNT(te.id) 
FROM technical_event te
JOIN monitored_application ma ON te.application_id = ma.id
GROUP BY ma.name;
```

### Reset Database

```bash
# Stop and remove container
docker compose down -v

# Recreate everything (postgres, migrations, setup, API)
docker compose up -d
```

### Manual Migrations

Add new migration file:

```bash
touch migrations/000003_add_new_feature.sql
```

Example content:

```sql
-- 000003_add_new_feature.sql
ALTER TABLE technical_event ADD COLUMN tags TEXT[];

CREATE INDEX idx_technical_event_tags ON technical_event USING GIN(tags);
```

Run migrations (only files not yet in `schema_migrations` are applied):

```bash
go run . migrate
```

---

## Dependencies

### Core Dependencies

- **[pgx/v5](https://github.com/jackc/pgx)**: PostgreSQL driver and toolkit
- **[cobra](https://github.com/spf13/cobra)**: CLI framework

### Adding Dependencies

```bash
go get github.com/some/package
go mod tidy
```

### Updating Dependencies

```bash
go get -u ./...
go mod tidy
```

---

## Debugging

### Enable Verbose Logging

Add debug output to use cases:

```go
fmt.Printf("[DEBUG] Input: %+v\n", input)
```

### Debug PostgreSQL Queries

Enable query logging in PostgreSQL:

```bash
docker compose exec postgres psql -U postgres -d tracelog -c \
  "ALTER SYSTEM SET log_statement = 'all';"
docker compose restart postgres
docker compose logs -f postgres
```

### Debug HTTP Requests

Use verbose curl:

```bash
curl -v -X POST http://localhost:8080/events \
  -H "Content-Type: application/json" \
  -d '{ ... }'
```

### Use Delve Debugger

Install Delve:

```bash
go install github.com/go-delve/delve/cmd/dlv@latest
```

Debug the application:

```bash
dlv debug . -- serve
```

Set breakpoints in code and debug interactively.

---

## Troubleshooting

### Port Already in Use

```
Error: listen tcp :8080: bind: address already in use
```

**Solution**: Change port or kill process using it:

```bash
# Find process
lsof -i :8080

# Kill process
kill -9 <PID>

# Or use different port
go run . serve --http-port 8081
```

### PostgreSQL Connection Failed

```
Error: failed to open PostgreSQL connection: connection refused
```

**Solution**: Ensure PostgreSQL is running:

```bash
docker compose up -d
docker compose ps
```

### Migration Errors

```
Error: relation "technical_event" already exists
```

**Solution**: Reset database or manually fix schema:

```bash
docker compose down -v   # deletes the database volume
docker compose up -d     # recreates schema and setup
```

---

## Best Practices

### Code Style

- Follow Go conventions: `gofmt`, `golint`, `go vet`
- Use meaningful variable names
- Keep functions small and focused
- Document exported functions and types

### Testing

- Write unit tests for use cases
- Mock repository interfaces for testing
- Test edge cases and error scenarios
- Aim for >80% coverage on core logic

### Git Commits

Follow conventional commits:

```
feat: add new feature
fix: resolve bug
docs: update documentation
refactor: restructure code
test: add tests
chore: update dependencies
```

### Code Organization

- Keep domain logic pure (no external dependencies)
- Use interfaces for all external interactions
- Separate concerns: handlers, use cases, repositories
- Follow single responsibility principle

---

## Resources

- **Go Documentation**: [https://go.dev/doc/](https://go.dev/doc/)
- **PostgreSQL Docs**: [https://www.postgresql.org/docs/](https://www.postgresql.org/docs/)
- **Hexagonal Architecture**: [https://alistair.cockburn.us/hexagonal-architecture/](https://alistair.cockburn.us/hexagonal-architecture/)

---

## Getting Help

- Check existing documentation in `docs/`
- Review similar implementations in codebase
- Open an issue on GitHub
- Ask questions in project discussions

---

**Last Updated**: March 2026  
**Contributors**: See GitHub contributors page
