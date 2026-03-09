package services

import (
	"context"
	"testing"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

type stubTechnicalEventRepository struct {
	saved *domain.TechnicalEvent
}

func (r *stubTechnicalEventRepository) Save(_ context.Context, event *domain.TechnicalEvent) error {
	r.saved = event
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
		OccurredAt:       time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		Message:          "Database timeout",
		IngestionChannel: domain.IngestionHTTP,
		ApplicationID:    "app-1",
		SourceID:         "source-1",
		EnvironmentID:    "env-1",
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
		OccurredAt:       time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		Message:          "worker stopped unexpectedly",
		Payload:          map[string]any{"stack_trace": "panic: nil pointer"},
		IngestionChannel: domain.IngestionHTTP,
		ApplicationID:    "app-1",
		SourceID:         "source-1",
		EnvironmentID:    "env-1",
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
		OccurredAt:       time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC),
		Message:          "user profile loaded",
		IngestionChannel: domain.IngestionHTTP,
		ApplicationID:    "app-1",
		SourceID:         "source-1",
		EnvironmentID:    "env-1",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if event.Severity != domain.SeverityInfo {
		t.Fatalf("event severity = %q, want %q", event.Severity, domain.SeverityInfo)
	}
}
