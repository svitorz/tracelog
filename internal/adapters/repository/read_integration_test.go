package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
	"svitorz/tracelog/internal/core/services"
)

// Testes de integração da leitura (spec 0005). Mesmas regras de integration_test.go.

func TestIntegrationPaginationWithEqualTimestamps(t *testing.T) {
	f := newIntegrationFixture(t)
	repo := NewPostgresTechnicalEventRepository(f.db)
	ctx := context.Background()

	same := time.Date(2026, 3, 10, 14, 47, 0, 123456000, time.UTC)
	want := map[string]bool{}
	for i := 0; i < 7; i++ {
		e := f.event(f.appA, "", "same", domain.SeverityError, same)
		if _, err := repo.Save(ctx, e); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		want[e.ID] = true
	}
	for _, d := range []int{9, 11} {
		e := f.event(f.appA, "", "other", domain.SeverityError, time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC))
		if _, err := repo.Save(ctx, e); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
		want[e.ID] = true
	}

	uc := services.NewListEventsPageUseCase(repo)
	limit := 3
	seen := map[string]int{}
	var previous *domain.TechnicalEvent
	var after *ports.EventCursor
	pages := 0
	for {
		page, err := uc.Execute(ctx, services.ListEventsPageInput{Limit: &limit, After: after})
		if err != nil {
			t.Fatalf("page %d error = %v", pages+1, err)
		}
		pages++
		for i := range page.Items {
			e := page.Items[i]
			seen[e.ID]++
			if previous != nil && (e.OccurredAt.After(previous.OccurredAt) || (e.OccurredAt.Equal(previous.OccurredAt) && e.ID >= previous.ID)) {
				t.Fatalf("order broken: %s/%s after %s/%s", e.OccurredAt, e.ID, previous.OccurredAt, previous.ID)
			}
			previous = &e
		}
		if page.Next == nil {
			break
		}
		after = page.Next
	}

	// 9 eventos com limit 3: a terceira página está exatamente cheia e não pode gerar uma quarta vazia.
	if pages != 3 {
		t.Fatalf("pages = %d, want 3", pages)
	}
	if len(seen) != len(want) {
		t.Fatalf("saw %d distinct events, want %d", len(seen), len(want))
	}
	for id, n := range seen {
		if !want[id] || n != 1 {
			t.Fatalf("event %s seen %d times (known: %v)", id, n, want[id])
		}
	}
}

func TestIntegrationCursorFromAnotherPositionStillPaginates(t *testing.T) {
	f := newIntegrationFixture(t)
	repo := NewPostgresTechnicalEventRepository(f.db)
	ctx := context.Background()
	for d := 1; d <= 3; d++ {
		if _, err := repo.Save(ctx, f.event(f.appA, "", "e", domain.SeverityInfo, time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC))); err != nil {
			t.Fatalf("Save() error = %v", err)
		}
	}

	// Cursor bem formado que não corresponde a nenhum evento: só posiciona a lista.
	got, err := repo.FindByFilters(ctx, ports.TechnicalEventFilters{
		Limit: 10,
		After: &ports.EventCursor{OccurredAt: time.Date(2026, 3, 2, 12, 0, 0, 0, time.UTC), ID: "ffffffff-ffff-4fff-bfff-ffffffffffff"},
	})
	if err != nil {
		t.Fatalf("FindByFilters() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want the 2 older than the cursor", len(got))
	}
}

func TestIntegrationFindByID(t *testing.T) {
	f := newIntegrationFixture(t)
	repo := NewPostgresTechnicalEventRepository(f.db)
	ctx := context.Background()

	e := f.event(f.appA, "ext-1", "<script>alert(1)</script>", domain.SeverityError, time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))
	e.Payload = map[string]any{"_ingestion": map[string]any{"normalization_warnings": []any{"x"}}}
	e.RawLine = "raw"
	if _, err := repo.Save(ctx, e); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	got, err := repo.FindByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("FindByID() error = %v", err)
	}
	if got.Message != e.Message || got.RawLine != "raw" || got.ExternalID != "ext-1" || got.Payload["_ingestion"] == nil {
		t.Fatalf("FindByID() = %+v", got)
	}

	_, err = repo.FindByID(ctx, "ffffffff-ffff-4fff-bfff-ffffffffffff")
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("FindByID(missing) error = %v, want ports.ErrNotFound", err)
	}
}

func TestIntegrationEventStats(t *testing.T) {
	f := newIntegrationFixture(t)
	events := NewPostgresTechnicalEventRepository(f.db)
	stats := NewPostgresEventStatsRepository(f.db)
	ctx := context.Background()

	empty, err := stats.DataRange(ctx)
	if err != nil || empty.FirstOccurredAt != nil || empty.LastOccurredAt != nil || empty.LastErrorAt != nil {
		t.Fatalf("DataRange(empty) = %+v, %v; want all nil", empty, err)
	}

	at := func(d, h int) time.Time { return time.Date(2026, 3, d, h, 0, 0, 0, time.UTC) }
	for _, e := range []*domain.TechnicalEvent{
		f.event(f.appA, "", "a", domain.SeverityInfo, at(1, 8)),
		f.event(f.appA, "", "b", domain.SeverityError, at(2, 9)),
		f.event(f.appA, "", "c", domain.SeverityFatal, at(3, 10)),
		f.event(f.appA, "", "d", domain.SeverityInfo, at(20, 11)),
		f.event(f.appB, "", "e", domain.SeverityWarn, at(2, 12)),
	} {
		if _, err := events.Save(ctx, e); err != nil {
			t.Fatalf("Save(%s) error = %v", e.Message, err)
		}
	}

	dr, err := stats.DataRange(ctx)
	if err != nil {
		t.Fatalf("DataRange() error = %v", err)
	}
	if !dr.FirstOccurredAt.Equal(at(1, 8)) || !dr.LastOccurredAt.Equal(at(20, 11)) || !dr.LastErrorAt.Equal(at(3, 10)) {
		t.Fatalf("DataRange() = %s, %s, %s", dr.FirstOccurredAt, dr.LastOccurredAt, dr.LastErrorAt)
	}

	from, to := at(1, 0), at(3, 23)
	counts, err := stats.CountBySeverity(ctx, ports.EventStatsFilters{ApplicationID: f.appA, OccurredFrom: &from, OccurredTo: &to})
	if err != nil {
		t.Fatalf("CountBySeverity() error = %v", err)
	}
	want := map[domain.Severity]int64{domain.SeverityInfo: 1, domain.SeverityError: 1, domain.SeverityFatal: 1}
	if len(counts) != len(want) {
		t.Fatalf("CountBySeverity() = %v, want %v", counts, want)
	}
	for k, v := range want {
		if counts[k] != v {
			t.Fatalf("CountBySeverity() = %v, want %v", counts, want)
		}
	}
}

func TestIntegrationReferenceData(t *testing.T) {
	f := newIntegrationFixture(t)
	repo := NewPostgresReferenceDataRepository(f.db)
	ctx := context.Background()
	mustQueryRow(t, f.db, `INSERT INTO environment (code, name, description) VALUES ('a-first', 'First', 'desc') RETURNING code`, new(string))

	apps, err := repo.ListApplications(ctx)
	if err != nil || len(apps) != 2 || apps[0].Name != "A" || apps[0].ServiceKey != "app-a" || apps[0].CreatedAt.IsZero() {
		t.Fatalf("ListApplications() = %+v, %v", apps, err)
	}

	envs, err := repo.ListEnvironments(ctx)
	if err != nil || len(envs) != 2 || envs[0].Code != "a-first" || envs[0].Description != "desc" || envs[1].Description != "" {
		t.Fatalf("ListEnvironments() = %+v, %v; want ordered by code", envs, err)
	}

	sources, err := repo.ListSources(ctx)
	if err != nil || len(sources) != 1 || sources[0].SourceType != "http" {
		t.Fatalf("ListSources() = %+v, %v", sources, err)
	}
}
