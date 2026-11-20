package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

type stubTechnicalEventRepository struct {
	saved      *domain.TechnicalEvent
	err        error
	saveErrors []error
	saves      int
	duplicate  bool
}

func (r *stubTechnicalEventRepository) Save(_ context.Context, event *domain.TechnicalEvent) (bool, error) {
	r.saves++
	r.saved = event
	if len(r.saveErrors) > 0 {
		err := r.saveErrors[0]
		r.saveErrors = r.saveErrors[1:]
		if err != nil {
			return false, err
		}
	}
	if r.err != nil {
		return false, r.err
	}
	event.ID = "event-1"
	return !r.duplicate, nil
}

func (r *stubTechnicalEventRepository) FindByFilters(context.Context, ports.TechnicalEventFilters) ([]domain.TechnicalEvent, error) {
	return nil, nil
}

func TestRegisterTechnicalEventDerivesErrorSeverityFromMessage(t *testing.T) {
	repo := &stubTechnicalEventRepository{}
	uc := NewRegisterTechnicalEventUseCase(repo)

	result, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
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
	event := result.Event

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

	result, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
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
	event := result.Event

	if event.Severity != domain.SeverityFatal {
		t.Fatalf("event severity = %q, want %q", event.Severity, domain.SeverityFatal)
	}
}

func TestRegisterTechnicalEventDefaultsSeverityToInfo(t *testing.T) {
	repo := &stubTechnicalEventRepository{}
	uc := NewRegisterTechnicalEventUseCase(repo)

	result, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
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
	event := result.Event

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

	result, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
		OccurredAt:       "not-a-date",
		Category:         "invalid-category",
		IngestionChannel: domain.IngestionHTTP,
		RawRequestBody:   `{"occurred_at":"not-a-date"}`,
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}
	event := result.Event

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
			fmt.Errorf("failed to save technical event: %w: application_id", ports.ErrReferenceNotFound),
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

	result, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
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
	event := result.Event

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

func TestRegisterTechnicalEventDoesNotRetryOnOtherErrors(t *testing.T) {
	repo := &stubTechnicalEventRepository{err: errors.New("connection refused")}
	uc := NewRegisterTechnicalEventUseCase(repo, &stubSystemSetupRepository{saved: basicSetupFixture()})

	_, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
		Message:          "x",
		IngestionChannel: domain.IngestionHTTP,
		ApplicationID:    "550e8400-e29b-41d4-a716-446655440000",
		SourceID:         "660e8400-e29b-41d4-a716-446655440001",
		EnvironmentID:    "770e8400-e29b-41d4-a716-446655440002",
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want the repository error")
	}
	if repo.saves != 1 {
		t.Fatalf("repo.Save calls = %d, want 1 (no fallback retry)", repo.saves)
	}
}

func TestRegisterTechnicalEventReportsDuplicate(t *testing.T) {
	repo := &stubTechnicalEventRepository{duplicate: true}
	uc := NewRegisterTechnicalEventUseCase(repo)

	result, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
		ExternalID:       "ext-1",
		Message:          "x",
		IngestionChannel: domain.IngestionHTTP,
		ApplicationID:    "550e8400-e29b-41d4-a716-446655440000",
		SourceID:         "660e8400-e29b-41d4-a716-446655440001",
		EnvironmentID:    "770e8400-e29b-41d4-a716-446655440002",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}
	if !result.Duplicate {
		t.Fatal("Duplicate = false, want true when the repository did not create a row")
	}
}

func TestRegisterTechnicalEventTruncatesOversizedIdentifiers(t *testing.T) {
	repo := &stubTechnicalEventRepository{}
	uc := NewRegisterTechnicalEventUseCase(repo)

	result, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
		ExternalID:       strings.Repeat("é", 300),
		TraceID:          strings.Repeat("t", 129),
		CorrelationID:    strings.Repeat("c", 128),
		Message:          "x",
		IngestionChannel: domain.IngestionHTTP,
		ApplicationID:    "550e8400-e29b-41d4-a716-446655440000",
		SourceID:         "660e8400-e29b-41d4-a716-446655440001",
		EnvironmentID:    "770e8400-e29b-41d4-a716-446655440002",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	event := result.Event
	if got := utf8.RuneCountInString(event.ExternalID); got != 255 {
		t.Fatalf("ExternalID length = %d characters, want 255", got)
	}
	if !utf8.ValidString(event.ExternalID) {
		t.Fatal("ExternalID was cut in the middle of a UTF-8 sequence")
	}
	if len(event.TraceID) != 128 {
		t.Fatalf("TraceID length = %d, want 128", len(event.TraceID))
	}
	if len(event.CorrelationID) != 128 {
		t.Fatalf("CorrelationID length = %d, want 128 (at the limit, untouched)", len(event.CorrelationID))
	}

	meta, _ := event.Payload["_ingestion"].(map[string]any)
	warnings, _ := meta["normalization_warnings"].([]string)
	joined := strings.Join(warnings, "; ")
	for _, want := range []string{"external_id truncated from 300 to 255", "trace_id truncated from 129 to 128"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("warnings = %q, want %q", joined, want)
		}
	}
	if strings.Contains(joined, "correlation_id truncated") {
		t.Fatalf("warnings = %q, correlation_id at the limit must not be reported", joined)
	}
}

func TestRegisterTechnicalEventKeepsRawBodyOnlyWhenUndecodable(t *testing.T) {
	tests := []struct {
		name        string
		decodeError string
		wantRawBody bool
	}{
		{"decoded body with warnings", "", false},
		{"undecodable body", "invalid character 'n'", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := NewRegisterTechnicalEventUseCase(&stubTechnicalEventRepository{}, &stubSystemSetupRepository{saved: basicSetupFixture()})

			result, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
				Message:          "x",
				IngestionChannel: domain.IngestionHTTP,
				RawRequestBody:   `{"message":"x"}`,
				DecodeError:      tt.decodeError,
			})
			if err != nil {
				t.Fatalf("Execute() unexpected error = %v", err)
			}

			meta, _ := result.Event.Payload["_ingestion"].(map[string]any)
			_, hasRaw := meta["raw_request_body"]
			if hasRaw != tt.wantRawBody {
				t.Fatalf("raw_request_body present = %v, want %v (_ingestion = %#v)", hasRaw, tt.wantRawBody, meta)
			}
		})
	}
}
