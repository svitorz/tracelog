package services

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// memoryEventRepository filters and orders like the PostgreSQL adapter
// (occurred_at DESC, id DESC, keyset on After), so pagination can be tested end to end.
type memoryEventRepository struct {
	events      []domain.TechnicalEvent
	lastFilters ports.TechnicalEventFilters
	err         error
}

func (r *memoryEventRepository) Save(context.Context, *domain.TechnicalEvent) (bool, error) {
	return true, nil
}

func (r *memoryEventRepository) FindByFilters(_ context.Context, f ports.TechnicalEventFilters) ([]domain.TechnicalEvent, error) {
	r.lastFilters = f
	if r.err != nil {
		return nil, r.err
	}

	sorted := append([]domain.TechnicalEvent(nil), r.events...)
	sort.Slice(sorted, func(i, j int) bool { return before(sorted[j], sorted[i].OccurredAt, sorted[i].ID) })

	out := []domain.TechnicalEvent{}
	for _, e := range sorted {
		if f.Severity != "" && e.Severity != f.Severity {
			continue
		}
		if f.After != nil && !before(e, f.After.OccurredAt, f.After.ID) {
			continue
		}
		out = append(out, e)
		if len(out) == f.Limit {
			break
		}
	}
	return out, nil
}

// before reports whether e comes after (t, id) in the descending order.
func before(e domain.TechnicalEvent, t time.Time, id string) bool {
	if !e.OccurredAt.Equal(t) {
		return e.OccurredAt.Before(t)
	}
	return e.ID < id
}

func (r *memoryEventRepository) FindByID(_ context.Context, id string) (domain.TechnicalEvent, error) {
	for _, e := range r.events {
		if e.ID == id {
			return e, nil
		}
	}
	return domain.TechnicalEvent{}, ports.ErrNotFound
}

func uuidN(n int) string {
	const base = "00000000-0000-4000-8000-000000000000"
	digits := []byte(base)
	for i := len(digits) - 1; n > 0; i-- {
		digits[i] = "0123456789abcdef"[n%16]
		n /= 16
	}
	return string(digits)
}

func intPtr(v int) *int { return &v }

func timePtr(t time.Time) *time.Time { return &t }

func TestListEventsPageWalksEqualTimestampsWithoutLossOrRepetition(t *testing.T) {
	same := time.Date(2026, 3, 10, 14, 0, 0, 0, time.UTC)
	repo := &memoryEventRepository{}
	for i := 1; i <= 7; i++ {
		repo.events = append(repo.events, domain.TechnicalEvent{ID: uuidN(i), OccurredAt: same, Severity: domain.SeverityError})
	}
	uc := NewListEventsPageUseCase(repo)

	seen := map[string]int{}
	var after *ports.EventCursor
	pages := 0
	for {
		page, err := uc.Execute(context.Background(), ListEventsPageInput{Limit: intPtr(3), After: after})
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}
		pages++
		for _, e := range page.Items {
			seen[e.ID]++
		}
		if page.Next == nil {
			break
		}
		after = page.Next
	}

	if pages != 3 || len(seen) != 7 {
		t.Fatalf("pages = %d, distinct events = %d; want 3 pages and 7 events", pages, len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Fatalf("event %s seen %d times", id, n)
		}
	}
}

func TestListEventsPageExactlyFullLastPageHasNoNextCursor(t *testing.T) {
	repo := &memoryEventRepository{}
	for i := 1; i <= 4; i++ {
		repo.events = append(repo.events, domain.TechnicalEvent{ID: uuidN(i), OccurredAt: time.Date(2026, 3, i, 0, 0, 0, 0, time.UTC)})
	}
	uc := NewListEventsPageUseCase(repo)

	first, err := uc.Execute(context.Background(), ListEventsPageInput{Limit: intPtr(2)})
	if err != nil || first.Next == nil {
		t.Fatalf("first page: err = %v, next = %v; want a next cursor", err, first.Next)
	}
	second, err := uc.Execute(context.Background(), ListEventsPageInput{Limit: intPtr(2), After: first.Next})
	if err != nil {
		t.Fatalf("second page error = %v", err)
	}
	if len(second.Items) != 2 || second.Next != nil {
		t.Fatalf("second page = %d items, next = %v; want 2 items and no cursor", len(second.Items), second.Next)
	}
	if repo.lastFilters.Limit != 3 {
		t.Fatalf("repository limit = %d, want limit+1 = 3", repo.lastFilters.Limit)
	}
}

func TestListEventsPageDefaultsAndNormalizes(t *testing.T) {
	repo := &memoryEventRepository{}
	uc := NewListEventsPageUseCase(repo)

	_, err := uc.Execute(context.Background(), ListEventsPageInput{
		Severity:   "ERROR",
		EventScope: EventScope{ApplicationID: " " + uuidN(1) + " "},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if repo.lastFilters.Limit != DefaultPageLimit+1 || repo.lastFilters.Severity != domain.SeverityError || repo.lastFilters.ApplicationID != uuidN(1) {
		t.Fatalf("filters = %+v", repo.lastFilters)
	}
}

func TestListEventsPageRejectsInvalidInput(t *testing.T) {
	from := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		input ListEventsPageInput
		field string
	}{
		{"limit zero", ListEventsPageInput{Limit: intPtr(0)}, "limit"},
		{"limit above max", ListEventsPageInput{Limit: intPtr(MaxPageLimit + 1)}, "limit"},
		{"severity", ListEventsPageInput{Severity: "critical"}, "severity"},
		{"application", ListEventsPageInput{EventScope: EventScope{ApplicationID: "xyz"}}, "application_id"},
		{"environment", ListEventsPageInput{EventScope: EventScope{EnvironmentID: "xyz"}}, "environment_id"},
		{"period", ListEventsPageInput{EventScope: EventScope{OccurredFrom: &from, OccurredTo: &to}}, "from"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &memoryEventRepository{}
			_, err := NewListEventsPageUseCase(repo).Execute(context.Background(), tt.input)
			assertValidationField(t, err, tt.field)
			if repo.lastFilters.Limit != 0 {
				t.Fatalf("repository was called with %+v", repo.lastFilters)
			}
		})
	}

	_, err := NewListEventsPageUseCase(&memoryEventRepository{}).Execute(context.Background(), ListEventsPageInput{Limit: intPtr(MaxPageLimit)})
	if err != nil {
		t.Fatalf("limit = max: error = %v", err)
	}
}

func TestListEventsPageWrapsRepositoryErrorsUntyped(t *testing.T) {
	_, err := NewListEventsPageUseCase(&memoryEventRepository{err: errors.New("db down")}).Execute(context.Background(), ListEventsPageInput{})
	var validation *ValidationError
	if err == nil || errors.As(err, &validation) {
		t.Fatalf("error = %v, want an untyped error", err)
	}
}

func assertValidationField(t *testing.T, err error, field string) {
	t.Helper()
	var validation *ValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %v, want *ValidationError", err)
	}
	if validation.Field != field {
		t.Fatalf("ValidationError.Field = %q, want %q (message %q)", validation.Field, field, validation.Message)
	}
}

func TestGetTechnicalEvent(t *testing.T) {
	repo := &memoryEventRepository{events: []domain.TechnicalEvent{{ID: uuidN(1), Message: "found"}}}
	uc := NewGetTechnicalEventUseCase(repo)

	got, err := uc.Execute(context.Background(), uuidN(1))
	if err != nil || got.Message != "found" {
		t.Fatalf("Execute(existing) = %+v, %v", got, err)
	}

	_, err = uc.Execute(context.Background(), uuidN(2))
	if !errors.Is(err, ports.ErrNotFound) {
		t.Fatalf("Execute(missing) error = %v, want ports.ErrNotFound", err)
	}

	_, err = uc.Execute(context.Background(), "abc")
	assertValidationField(t, err, "id")
}

type stubFrequencyRepository struct {
	rows    []domain.ErrorFrequency
	filters ports.ErrorFrequencyFilters
	calls   int
}

func (r *stubFrequencyRepository) GetErrorFrequencyByDay(_ context.Context, f ports.ErrorFrequencyFilters) ([]domain.ErrorFrequency, error) {
	r.calls++
	r.filters = f
	return r.rows, nil
}

func TestDailyErrorFrequencyFillsEmptyDaysInUTC(t *testing.T) {
	day := func(d int) time.Time { return time.Date(2026, 3, d, 0, 0, 0, 0, time.UTC) }
	repo := &stubFrequencyRepository{rows: []domain.ErrorFrequency{
		{Date: day(1), Errors: 2},
		{Date: day(3), Errors: 1, Fatal: 1},
	}}
	// 21:00 em São Paulo do dia 3 é dia 4 em UTC: o último dia tem de ser o 4.
	saoPaulo := time.FixedZone("BRT", -3*60*60)
	from := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 3, 3, 21, 0, 0, 0, saoPaulo)

	got, err := NewGetDailyErrorFrequencyUseCase(repo).Execute(context.Background(), EventScope{OccurredFrom: &from, OccurredTo: &to})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := []domain.ErrorFrequency{{Date: day(1), Errors: 2}, {Date: day(2)}, {Date: day(3), Errors: 1, Fatal: 1}, {Date: day(4)}}
	if len(got.Days) != len(want) {
		t.Fatalf("days = %+v, want %+v", got.Days, want)
	}
	for i := range want {
		if !got.Days[i].Date.Equal(want[i].Date) || got.Days[i].Errors != want[i].Errors || got.Days[i].Fatal != want[i].Fatal {
			t.Fatalf("day %d = %+v, want %+v", i, got.Days[i], want[i])
		}
	}
	if !got.From.Equal(day(1)) || !got.To.Equal(day(4)) {
		t.Fatalf("From/To = %s/%s", got.From, got.To)
	}
	if !repo.filters.OccurredTo.Equal(to) {
		t.Fatalf("counts must use the exact instant: to = %s", repo.filters.OccurredTo)
	}
}

func TestDailyErrorFrequencyValidation(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	maxTo := base.AddDate(0, 0, MaxFrequencyDays-1)
	overTo := base.AddDate(0, 0, MaxFrequencyDays)

	tests := []struct {
		name  string
		input EventScope
		field string
	}{
		{"missing from", EventScope{OccurredTo: &base}, "from"},
		{"missing to", EventScope{OccurredFrom: &base}, "to"},
		{"too long", EventScope{OccurredFrom: &base, OccurredTo: &overTo}, "to"},
		{"from after to", EventScope{OccurredFrom: &overTo, OccurredTo: &base}, "from"},
		{"bad application", EventScope{OccurredFrom: &base, OccurredTo: &base, ApplicationID: "x"}, "application_id"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubFrequencyRepository{}
			_, err := NewGetDailyErrorFrequencyUseCase(repo).Execute(context.Background(), tt.input)
			assertValidationField(t, err, tt.field)
			if repo.calls != 0 {
				t.Fatal("repository must not be called on invalid input")
			}
		})
	}

	got, err := NewGetDailyErrorFrequencyUseCase(&stubFrequencyRepository{}).Execute(context.Background(), EventScope{OccurredFrom: &base, OccurredTo: &maxTo})
	if err != nil || len(got.Days) != MaxFrequencyDays {
		t.Fatalf("max period: %d days, err = %v; want %d days", len(got.Days), err, MaxFrequencyDays)
	}
}

type stubStatsRepository struct {
	counts  map[domain.Severity]int64
	dr      ports.EventDataRange
	filters ports.EventStatsFilters
}

func (r *stubStatsRepository) CountBySeverity(_ context.Context, f ports.EventStatsFilters) (map[domain.Severity]int64, error) {
	r.filters = f
	return r.counts, nil
}

func (r *stubStatsRepository) DataRange(context.Context) (ports.EventDataRange, error) {
	return r.dr, nil
}

func TestSeverityCountsIncludesEverySeverity(t *testing.T) {
	repo := &stubStatsRepository{counts: map[domain.Severity]int64{domain.SeverityInfo: 3, domain.SeverityFatal: 1}}

	got, err := NewGetSeverityCountsUseCase(repo).Execute(context.Background(), EventScope{EnvironmentID: uuidN(9)})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(got.Counts) != len(AllSeverities) || got.Counts[domain.SeverityDebug] != 0 || got.Counts[domain.SeverityInfo] != 3 || got.Total != 4 {
		t.Fatalf("counts = %+v", got)
	}
	if repo.filters.EnvironmentID != uuidN(9) {
		t.Fatalf("filters = %+v", repo.filters)
	}

	_, err = NewGetSeverityCountsUseCase(repo).Execute(context.Background(), EventScope{ApplicationID: "nope"})
	assertValidationField(t, err, "application_id")
}

func TestDataRangeDefaultPeriod(t *testing.T) {
	now := time.Date(2026, 11, 23, 20, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	date := func(m time.Month, d int) time.Time { return time.Date(2026, m, d, 0, 0, 0, 0, time.UTC) }

	tests := []struct {
		name     string
		stored   ports.EventDataRange
		from, to time.Time
	}{
		{
			// O dataset de exemplo: erros até 14/03 e eventos sem timestamp na data do import.
			name: "ends on the latest error",
			stored: ports.EventDataRange{
				FirstOccurredAt: timePtr(time.Date(2026, 3, 1, 8, 46, 9, 0, time.UTC)),
				LastOccurredAt:  timePtr(now),
				LastErrorAt:     timePtr(time.Date(2026, 3, 14, 21, 12, 0, 0, time.UTC)),
			},
			from: date(3, 1), to: date(3, 14),
		},
		{
			name:   "no errors: ends on the latest event",
			stored: ports.EventDataRange{FirstOccurredAt: timePtr(date(5, 1)), LastOccurredAt: timePtr(date(5, 20).Add(3 * time.Hour))},
			from:   date(5, 7), to: date(5, 20),
		},
		{name: "no events: ends today", from: date(11, 10), to: date(11, 23)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewGetDataRangeUseCase(&stubStatsRepository{dr: tt.stored}, clock).Execute(context.Background())
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !got.DefaultFrom.Equal(tt.from) || !got.DefaultTo.Equal(tt.to) {
				t.Fatalf("default = %s..%s, want %s..%s", got.DefaultFrom.Format(time.DateOnly), got.DefaultTo.Format(time.DateOnly), tt.from.Format(time.DateOnly), tt.to.Format(time.DateOnly))
			}
		})
	}
}

type stubReferenceRepository struct{}

func (stubReferenceRepository) ListApplications(context.Context) ([]domain.MonitoredApplication, error) {
	return []domain.MonitoredApplication{{Name: "A"}}, nil
}

func (stubReferenceRepository) ListEnvironments(context.Context) ([]domain.Environment, error) {
	return nil, errors.New("db down")
}

func (stubReferenceRepository) ListSources(context.Context) ([]domain.EventSource, error) {
	return []domain.EventSource{}, nil
}

func TestListReferenceData(t *testing.T) {
	uc := NewListReferenceDataUseCase(stubReferenceRepository{})

	apps, err := uc.Applications(context.Background())
	if err != nil || len(apps) != 1 {
		t.Fatalf("Applications() = %v, %v", apps, err)
	}
	if _, err := uc.Environments(context.Background()); err == nil {
		t.Fatal("Environments() error = nil, want the repository error")
	}
	if sources, err := uc.Sources(context.Background()); err != nil || sources == nil {
		t.Fatalf("Sources() = %v, %v", sources, err)
	}
}
