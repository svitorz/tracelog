package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

type stubTechnicalEventRepository struct {
	saved      *domain.TechnicalEvent
	err        error
	saveErrors []error
	saves      int
}

func (r *stubTechnicalEventRepository) Save(_ context.Context, event *domain.TechnicalEvent) error {
	r.saves++
	r.saved = event
	if len(r.saveErrors) > 0 {
		err := r.saveErrors[0]
		r.saveErrors = r.saveErrors[1:]
		if err != nil {
			return err
		}
	}
	if r.err != nil {
		return r.err
	}
	event.ID = "event-1"
	return nil
}

func (r *stubTechnicalEventRepository) FindByFilters(context.Context, ports.TechnicalEventFilters) ([]domain.TechnicalEvent, error) {
	return nil, nil
}

func TestRegisterTechnicalEventDerivesErrorSeverityFromMessage(t *testing.T) {
	repo := &stubTechnicalEventRepository{}
	uc := NewRegisterTechnicalEventUseCase(repo)

	event, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
		ExternalID:       "ext-1",
		OccurredAt:       "2026-03-09T12:00:00Z",
		Message:          "Database timeout",
		IngestionChannel: domain.IngestionHTTP,
		ApplicationID:    "550e8400-e29b-41d4-a716-446655440000",
		SourceID:         "660e8400-e29b-41d4-a716-446655440001",
		EnvironmentID:    "770e8400-e29b-41d4-a716-446655440002",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if event.Severity != domain.SeverityError {
		t.Fatalf("event severity = %q, want %q", event.Severity, domain.SeverityError)
	}

	if repo.saved == nil || repo.saved.Severity != domain.SeverityError {
		t.Fatalf("saved severity = %q, want %q", repo.saved.Severity, domain.SeverityError)
	}
}

func TestRegisterTechnicalEventDerivesFatalSeverityFromPayloadSignal(t *testing.T) {
	repo := &stubTechnicalEventRepository{}
	uc := NewRegisterTechnicalEventUseCase(repo)

	event, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
		ExternalID:       "ext-1",
		OccurredAt:       "2026-03-09T12:00:00Z",
		Message:          "worker stopped unexpectedly",
		Payload:          map[string]any{"stack_trace": "panic: nil pointer"},
		IngestionChannel: domain.IngestionHTTP,
		ApplicationID:    "550e8400-e29b-41d4-a716-446655440000",
		SourceID:         "660e8400-e29b-41d4-a716-446655440001",
		EnvironmentID:    "770e8400-e29b-41d4-a716-446655440002",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if event.Severity != domain.SeverityFatal {
		t.Fatalf("event severity = %q, want %q", event.Severity, domain.SeverityFatal)
	}
}

func TestRegisterTechnicalEventDefaultsSeverityToInfo(t *testing.T) {
	repo := &stubTechnicalEventRepository{}
	uc := NewRegisterTechnicalEventUseCase(repo)

	event, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
		ExternalID:       "ext-1",
		OccurredAt:       "2026-03-09T12:00:00Z",
		Message:          "user profile loaded",
		IngestionChannel: domain.IngestionHTTP,
		ApplicationID:    "550e8400-e29b-41d4-a716-446655440000",
		SourceID:         "660e8400-e29b-41d4-a716-446655440001",
		EnvironmentID:    "770e8400-e29b-41d4-a716-446655440002",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if event.Severity != domain.SeverityInfo {
		t.Fatalf("event severity = %q, want %q", event.Severity, domain.SeverityInfo)
	}
}

func TestRegisterTechnicalEventUsesFallbacksAndStoresNormalizationWarnings(t *testing.T) {
	repo := &stubTechnicalEventRepository{}
	setupRepo := &stubSystemSetupRepository{
		saved: ports.BasicSystemSetup{
			Application: domain.MonitoredApplication{ID: "550e8400-e29b-41d4-a716-446655440010"},
			Environment: domain.Environment{ID: "770e8400-e29b-41d4-a716-446655440012"},
			Source:      domain.EventSource{ID: "660e8400-e29b-41d4-a716-446655440011"},
		},
	}
	uc := NewRegisterTechnicalEventUseCase(repo, setupRepo)
	now := time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)
	uc.now = func() time.Time { return now }

	event, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
		OccurredAt:       "not-a-date",
		Category:         "invalid-category",
		IngestionChannel: domain.IngestionHTTP,
		RawRequestBody:   `{"occurred_at":"not-a-date"}`,
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if event.OccurredAt != now {
		t.Fatalf("event.OccurredAt = %v, want %v", event.OccurredAt, now)
	}
	if event.Category != domain.CategoryUnknown {
		t.Fatalf("event.Category = %q, want %q", event.Category, domain.CategoryUnknown)
	}
	if event.Message != "event received without message" {
		t.Fatalf("event.Message = %q, want fallback message", event.Message)
	}
	if event.ApplicationID != setupRepo.saved.Application.ID {
		t.Fatalf("event.ApplicationID = %q, want %q", event.ApplicationID, setupRepo.saved.Application.ID)
	}

	meta, ok := event.Payload["_ingestion"].(map[string]any)
	if !ok {
		t.Fatalf("payload _ingestion metadata missing: %#v", event.Payload)
	}

	warnings, ok := meta["normalization_warnings"].([]string)
	if !ok || len(warnings) == 0 {
		t.Fatalf("normalization warnings missing: %#v", meta["normalization_warnings"])
	}
}

func TestRegisterTechnicalEventRetriesWithFallbackSetupAfterForeignKeyFailure(t *testing.T) {
	repo := &stubTechnicalEventRepository{
		saveErrors: []error{
			errors.New("failed to save technical event: application_id does not reference an existing application"),
			nil,
		},
	}
	setupRepo := &stubSystemSetupRepository{
		saved: ports.BasicSystemSetup{
			Application: domain.MonitoredApplication{ID: "550e8400-e29b-41d4-a716-446655440010"},
			Environment: domain.Environment{ID: "770e8400-e29b-41d4-a716-446655440012"},
			Source:      domain.EventSource{ID: "660e8400-e29b-41d4-a716-446655440011"},
		},
	}
	uc := NewRegisterTechnicalEventUseCase(repo, setupRepo)

	event, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
		OccurredAt:       "2026-03-09T12:00:00Z",
		Message:          "foreign key mismatch",
		IngestionChannel: domain.IngestionHTTP,
		ApplicationID:    "550e8400-e29b-41d4-a716-446655440000",
		SourceID:         "660e8400-e29b-41d4-a716-446655440001",
		EnvironmentID:    "770e8400-e29b-41d4-a716-446655440002",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if repo.saves != 2 {
		t.Fatalf("repo.Save calls = %d, want 2", repo.saves)
	}
	if event.ApplicationID != setupRepo.saved.Application.ID {
		t.Fatalf("event.ApplicationID = %q, want fallback %q", event.ApplicationID, setupRepo.saved.Application.ID)
	}

	meta, ok := event.Payload["_ingestion"].(map[string]any)
	if !ok {
		t.Fatalf("payload _ingestion metadata missing: %#v", event.Payload)
	}
	warnings, ok := meta["normalization_warnings"].([]string)
	if !ok || len(warnings) == 0 {
		t.Fatalf("retry warnings missing: %#v", meta["normalization_warnings"])
	}
}
