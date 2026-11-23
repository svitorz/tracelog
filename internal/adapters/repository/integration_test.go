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
	"io/fs"
	"net/url"
	"os"
	"sort"
	"testing"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
	"svitorz/tracelog/migrations"
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

	db := newEmptySchema(t)
	ctx := context.Background()
	if _, err := Migrate(ctx, db, migrations.Files); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	f := integrationFixture{db: db}
	mustQueryRow(t, db, `INSERT INTO environment (code, name) VALUES ('it', 'IT') RETURNING id`, &f.envID)
	mustQueryRow(t, db, `INSERT INTO monitored_application (name, service_key) VALUES ('A', 'app-a') RETURNING id`, &f.appA)
	mustQueryRow(t, db, `INSERT INTO monitored_application (name, service_key) VALUES ('B', 'app-b') RETURNING id`, &f.appB)
	mustQueryRow(t, db, `INSERT INTO event_source (name, source_type) VALUES ('HTTP', 'http') RETURNING id`, &f.source)

	return f
}

// newEmptySchema returns a connection whose search_path is a new schema, dropped at cleanup.
func newEmptySchema(t *testing.T) *sql.DB {
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
	// Fuso diferente de UTC, para provar que a agregação por dia não depende da sessão.
	query.Set("timezone", "America/Sao_Paulo")
	parsed.RawQuery = query.Encode()

	db, err := sql.Open("pgx", parsed.String())
	if err != nil {
		t.Fatalf("open schema connection: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	return db
}

// applyRawMigrations reproduces the old scripts/run-migrations.sh: every file, twice,
// without schema_migrations. The double pass also checks the files are idempotent.
func applyRawMigrations(t *testing.T, db *sql.DB) {
	t.Helper()

	names, err := fs.Glob(migrations.Files, "*.sql")
	if err != nil || len(names) == 0 {
		t.Fatalf("find migrations: %v (found %d)", err, len(names))
	}
	sort.Strings(names)

	for pass := 1; pass <= 2; pass++ {
		for _, name := range names {
			content, err := fs.ReadFile(migrations.Files, name)
			if err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			if _, err := db.Exec(string(content)); err != nil {
				t.Fatalf("apply %s (pass %d): %v", name, pass, err)
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

func TestIntegrationErrorFrequencyByDay(t *testing.T) {
	f := newIntegrationFixture(t)
	events := NewPostgresTechnicalEventRepository(f.db)
	metrics := NewPostgresErrorFrequencyRepository(f.db)
	ctx := context.Background()
	at := func(day, hour int) time.Time { return time.Date(2026, 3, day, hour, 30, 0, 0, time.UTC) }

	for _, e := range []*domain.TechnicalEvent{
		f.event(f.appA, "", "e1", domain.SeverityError, at(1, 10)),
		f.event(f.appA, "", "f1", domain.SeverityFatal, at(1, 11)),
		f.event(f.appA, "", "i1", domain.SeverityInfo, at(1, 12)),
		// 02:30 UTC do dia 2 = 23:30 do dia 1 em São Paulo: deve contar no dia 2.
		f.event(f.appA, "", "e2", domain.SeverityError, at(2, 2)),
		f.event(f.appB, "", "e-other-app", domain.SeverityError, at(2, 3)),
		f.event(f.appA, "", "w3", domain.SeverityWarn, at(3, 10)),
	} {
		if _, err := events.Save(ctx, e); err != nil {
			t.Fatalf("Save(%s) error = %v", e.Message, err)
		}
	}

	got, err := metrics.GetErrorFrequencyByDay(ctx, ports.ErrorFrequencyFilters{ApplicationID: f.appA})
	if err != nil {
		t.Fatalf("GetErrorFrequencyByDay() error = %v", err)
	}

	want := []struct {
		date          string
		errors, fatal int64
	}{
		{"2026-03-01", 1, 1},
		{"2026-03-02", 1, 0},
	}
	if len(got) != len(want) {
		t.Fatalf("GetErrorFrequencyByDay() = %+v, want %d days (day 3 has only warn)", got, len(want))
	}
	for i, w := range want {
		if d := got[i].Date.Format("2006-01-02"); d != w.date || got[i].Errors != w.errors || got[i].Fatal != w.fatal {
			t.Errorf("day %d = %s errors=%d fatal=%d, want %s errors=%d fatal=%d", i, d, got[i].Errors, got[i].Fatal, w.date, w.errors, w.fatal)
		}
	}
}

func TestIntegrationMigrateAppliesEachVersionOnce(t *testing.T) {
	db := newEmptySchema(t)
	ctx := context.Background()

	first, err := Migrate(ctx, db, migrations.Files)
	if err != nil {
		t.Fatalf("first Migrate() error = %v", err)
	}
	if len(first.Applied) != first.Total || first.Total == 0 {
		t.Fatalf("first Migrate() applied %d of %d, want all", len(first.Applied), first.Total)
	}

	second, err := Migrate(ctx, db, migrations.Files)
	if err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
	if len(second.Applied) != 0 {
		t.Fatalf("second Migrate() applied %v, want nothing", second.Applied)
	}

	var recorded int
	mustQueryRow(t, db, `SELECT COUNT(*) FROM schema_migrations`, &recorded)
	if recorded != first.Total {
		t.Fatalf("schema_migrations rows = %d, want %d", recorded, first.Total)
	}
}

func TestIntegrationMigrateAdoptsDatabaseFromOldScript(t *testing.T) {
	db := newEmptySchema(t)
	applyRawMigrations(t, db)

	result, err := Migrate(context.Background(), db, migrations.Files)
	if err != nil {
		t.Fatalf("Migrate() on a database migrated by the old script error = %v", err)
	}
	if len(result.Applied) != result.Total {
		t.Fatalf("Migrate() applied %d of %d, want every version recorded", len(result.Applied), result.Total)
	}
}

func TestIntegrationSaveBasicSetupIsIdempotent(t *testing.T) {
	db := newEmptySchema(t)
	ctx := context.Background()
	if _, err := Migrate(ctx, db, migrations.Files); err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	repo := NewPostgresSystemSetupRepository(db)

	setup := ports.BasicSystemSetup{
		Application: domain.MonitoredApplication{Name: "Local Application", ServiceKey: "local-app"},
		Environment: domain.Environment{Code: "local", Name: "Local"},
		Source:      domain.EventSource{Name: "HTTP API", SourceType: "http"},
	}

	first, err := repo.SaveBasicSetup(ctx, setup)
	if err != nil {
		t.Fatalf("first SaveBasicSetup() error = %v", err)
	}
	second, err := repo.SaveBasicSetup(ctx, setup)
	if err != nil {
		t.Fatalf("second SaveBasicSetup() error = %v", err)
	}

	if first.Application.ID != second.Application.ID || first.Environment.ID != second.Environment.ID || first.Source.ID != second.Source.ID {
		t.Fatalf("IDs changed between runs: %+v vs %+v", first, second)
	}

	var apps, envs, sources int
	mustQueryRow(t, db, `SELECT COUNT(*) FROM monitored_application`, &apps)
	mustQueryRow(t, db, `SELECT COUNT(*) FROM environment`, &envs)
	mustQueryRow(t, db, `SELECT COUNT(*) FROM event_source`, &sources)
	if apps != 1 || envs != 1 || sources != 1 {
		t.Fatalf("rows = %d apps, %d envs, %d sources; want 1 each", apps, envs, sources)
	}
}
