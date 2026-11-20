package repository

// Testes de integração contra um PostgreSQL real. Só rodam com TRACELOG_TEST_DATABASE_URL
// definida, para que `go test ./...` continue funcionando sem banco. Cada teste cria um
// schema próprio, aplica migrations/*.sql nele e o remove no fim: nenhuma tabela
// existente do banco apontado é tocada.
//
// Exemplo:
//   TRACELOG_TEST_DATABASE_URL='postgres://postgres:postgres@127.0.0.1:5432/tracelog?sslmode=disable' go test ./internal/adapters/repository/

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

const integrationDBEnv = "TRACELOG_TEST_DATABASE_URL"

type integrationFixture struct {
	db     *sql.DB
	appA   string
	appB   string
	envID  string
	source string
}

func newIntegrationFixture(t *testing.T) integrationFixture {
	t.Helper()

	baseURL := os.Getenv(integrationDBEnv)
	if baseURL == "" {
		t.Skipf("%s not set; skipping PostgreSQL integration test", integrationDBEnv)
	}

	ctx := context.Background()
	admin, err := sql.Open("pgx", baseURL)
	if err != nil {
		t.Fatalf("open admin connection: %v", err)
	}
	t.Cleanup(func() { _ = admin.Close() })

	schema := fmt.Sprintf("tracelog_it_%d", time.Now().UnixNano())
	if _, err := admin.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.ExecContext(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Errorf("drop schema %s: %v", schema, err)
		}
	})

	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("%s must be a URL: %v", integrationDBEnv, err)
	}
	query := parsed.Query()
	query.Set("search_path", schema)
	parsed.RawQuery = query.Encode()

	db, err := sql.Open("pgx", parsed.String())
	if err != nil {
		t.Fatalf("open schema connection: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	applyMigrations(t, db)

	f := integrationFixture{db: db}
	mustQueryRow(t, db, `INSERT INTO environment (code, name) VALUES ('it', 'IT') RETURNING id`, &f.envID)
	mustQueryRow(t, db, `INSERT INTO monitored_application (name, service_key) VALUES ('A', 'app-a') RETURNING id`, &f.appA)
	mustQueryRow(t, db, `INSERT INTO monitored_application (name, service_key) VALUES ('B', 'app-b') RETURNING id`, &f.appB)
	mustQueryRow(t, db, `INSERT INTO event_source (name, source_type) VALUES ('HTTP', 'http') RETURNING id`, &f.source)

	return f
}

func applyMigrations(t *testing.T, db *sql.DB) {
	t.Helper()

	files, err := filepath.Glob(filepath.Join("..", "..", "..", "migrations", "*.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("find migrations: %v (found %d)", err, len(files))
	}
	sort.Strings(files)

	// Duas passadas: as migrations precisam ser idempotentes (AGENTS.md, seção 5).
	for pass := 1; pass <= 2; pass++ {
		for _, file := range files {
			content, err := os.ReadFile(file)
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			if _, err := db.Exec(string(content)); err != nil {
				t.Fatalf("apply %s (pass %d): %v", filepath.Base(file), pass, err)
			}
		}
	}
}

func mustQueryRow(t *testing.T, db *sql.DB, query string, dest any) {
	t.Helper()
	if err := db.QueryRow(query).Scan(dest); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func (f integrationFixture) event(app, externalID, message string, severity domain.Severity, occurredAt time.Time) *domain.TechnicalEvent {
	return &domain.TechnicalEvent{
		ExternalID:          externalID,
		OccurredAt:          occurredAt,
		Message:             message,
		Severity:            severity,
		Category:            domain.CategoryApplication,
		NormalizationStatus: domain.NormalizationStatusNormalized,
		IngestionChannel:    domain.IngestionHTTP,
		ApplicationID:       app,
		SourceID:            f.source,
		EnvironmentID:       f.envID,
	}
}

func (f integrationFixture) countEvents(t *testing.T) int {
	t.Helper()
	var n int
	mustQueryRow(t, f.db, `SELECT COUNT(*) FROM technical_event`, &n)
	return n
}

func TestIntegrationSaveDeduplicatesPerApplication(t *testing.T) {
	f := newIntegrationFixture(t)
	repo := NewPostgresTechnicalEventRepository(f.db)
	ctx := context.Background()
	at := time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)

	first := f.event(f.appA, "order-42", "first", domain.SeverityInfo, at)
	created, err := repo.Save(ctx, first)
	if err != nil || !created {
		t.Fatalf("first Save() = (%v, %v), want (true, nil)", created, err)
	}

	resend := f.event(f.appA, "order-42", "resent with other content", domain.SeverityError, at)
	created, err = repo.Save(ctx, resend)
	if err != nil || created {
		t.Fatalf("resend Save() = (%v, %v), want (false, nil)", created, err)
	}
	if resend.ID != first.ID || resend.Message != "first" {
		t.Fatalf("resend returned (%s, %q), want the stored event (%s, %q)", resend.ID, resend.Message, first.ID, "first")
	}

	otherApp := f.event(f.appB, "order-42", "same external_id, other app", domain.SeverityInfo, at)
	if created, err := repo.Save(ctx, otherApp); err != nil || !created {
		t.Fatalf("other application Save() = (%v, %v), want (true, nil)", created, err)
	}

	for i := 0; i < 2; i++ {
		if created, err := repo.Save(ctx, f.event(f.appA, "", "no external_id", domain.SeverityInfo, at)); err != nil || !created {
			t.Fatalf("Save() without external_id = (%v, %v), want (true, nil)", created, err)
		}
	}

	if got := f.countEvents(t); got != 4 {
		t.Fatalf("rows = %d, want 4", got)
	}
}

func TestIntegrationSaveReportsMissingReference(t *testing.T) {
	f := newIntegrationFixture(t)
	repo := NewPostgresTechnicalEventRepository(f.db)

	event := f.event("11111111-1111-4111-8111-111111111111", "x", "x", domain.SeverityInfo, time.Now())
	_, err := repo.Save(context.Background(), event)
	if !errors.Is(err, ports.ErrReferenceNotFound) {
		t.Fatalf("Save() error = %v, want ports.ErrReferenceNotFound", err)
	}
}

func TestIntegrationFindByFilters(t *testing.T) {
	f := newIntegrationFixture(t)
	repo := NewPostgresTechnicalEventRepository(f.db)
	ctx := context.Background()
	day := func(d int) time.Time { return time.Date(2026, 3, d, 10, 0, 0, 0, time.UTC) }

	for _, e := range []*domain.TechnicalEvent{
		f.event(f.appA, "", "a1", domain.SeverityError, day(1)),
		f.event(f.appA, "", "a2", domain.SeverityInfo, day(2)),
		f.event(f.appA, "", "a3", domain.SeverityError, day(3)),
		f.event(f.appB, "", "b1", domain.SeverityError, day(2)),
	} {
		if _, err := repo.Save(ctx, e); err != nil {
			t.Fatalf("Save(%s) error = %v", e.Message, err)
		}
	}

	from, to := day(2), day(3)
	got, err := repo.FindByFilters(ctx, ports.TechnicalEventFilters{
		ApplicationID: f.appA,
		Severity:      domain.SeverityError,
		OccurredFrom:  &from,
		OccurredTo:    &to,
	})
	if err != nil {
		t.Fatalf("FindByFilters() error = %v", err)
	}
	if len(got) != 1 || got[0].Message != "a3" {
		t.Fatalf("FindByFilters() = %d events (%v), want only a3", len(got), messages(got))
	}

	all, err := repo.FindByFilters(ctx, ports.TechnicalEventFilters{Limit: 2})
	if err != nil {
		t.Fatalf("FindByFilters(limit) error = %v", err)
	}
	if len(all) != 2 || all[0].Message != "a3" {
		t.Fatalf("FindByFilters(limit 2) = %v, want 2 events newest first", messages(all))
	}
}

func messages(events []domain.TechnicalEvent) []string {
	out := make([]string, 0, len(events))
	for _, e := range events {
		out = append(out, e.Message)
	}
	return out
}
