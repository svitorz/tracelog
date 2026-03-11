package services

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

func TestImportLogFilesUseCasePersistsMultipleNormalizationOutcomes(t *testing.T) {
	repo := &stubTechnicalEventRepository{}
	setupRepo := &stubSystemSetupRepository{
		saved: basicSetupFixture(),
	}
	registerUC := NewRegisterTechnicalEventUseCase(repo, setupRepo)
	registerUC.now = func() time.Time {
		return time.Date(2026, 3, 11, 15, 0, 0, 0, time.UTC)
	}

	importUC := NewImportLogFilesUseCase(registerUC)
	importUC.openFile = func(path string) (io.ReadCloser, error) {
		content := strings.Join([]string{
			`{"timestamp":"2026-03-09T12:00:00Z","message":"database timeout","trace_id":"trace-1"}`,
			`2026-03-09T12:01:00Z ERROR authentication denied`,
			`plain text without known structure`,
		}, "\n")
		return io.NopCloser(strings.NewReader(content)), nil
	}

	result, err := importUC.Execute(context.Background(), ImportLogFilesInput{
		Files: []string{"app.log"},
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if result.FilesProcessed != 1 {
		t.Fatalf("FilesProcessed = %d, want 1", result.FilesProcessed)
	}
	if result.LinesRead != 3 {
		t.Fatalf("LinesRead = %d, want 3", result.LinesRead)
	}
	if result.Persisted != 3 {
		t.Fatalf("Persisted = %d, want 3", result.Persisted)
	}
	if result.Normalized != 2 {
		t.Fatalf("Normalized = %d, want 2", result.Normalized)
	}
	if result.Uncategorizable != 1 {
		t.Fatalf("Uncategorizable = %d, want 1", result.Uncategorizable)
	}
}

func TestImportLogFilesUseCaseMarksBrokenJSONAsPartial(t *testing.T) {
	repo := &stubTechnicalEventRepository{}
	setupRepo := &stubSystemSetupRepository{
		saved: basicSetupFixture(),
	}
	registerUC := NewRegisterTechnicalEventUseCase(repo, setupRepo)
	registerUC.now = func() time.Time {
		return time.Date(2026, 3, 11, 15, 0, 0, 0, time.UTC)
	}

	importUC := NewImportLogFilesUseCase(registerUC)
	importUC.openFile = func(path string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(`{"message":"broken"`)), nil
	}

	result, err := importUC.Execute(context.Background(), ImportLogFilesInput{
		Files: []string{"broken.log"},
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if result.Partial != 1 {
		t.Fatalf("Partial = %d, want 1", result.Partial)
	}
	if repo.saved == nil {
		t.Fatal("repo.saved = nil, want persisted event")
	}
	if repo.saved.ParseError == "" {
		t.Fatal("ParseError = empty, want parser failure to be preserved")
	}
	if repo.saved.NormalizationStatus != domain.NormalizationStatusPartial {
		t.Fatalf("NormalizationStatus = %q, want %q", repo.saved.NormalizationStatus, domain.NormalizationStatusPartial)
	}
}

func TestRegisterTechnicalEventUsesImportedLevelForSeverity(t *testing.T) {
	repo := &stubTechnicalEventRepository{}
	uc := NewRegisterTechnicalEventUseCase(repo)

	event, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
		OccurredAt:          "2026-03-09T12:00:00Z",
		Message:             "worker completed",
		Payload:             map[string]any{"level": "debug"},
		NormalizationStatus: domain.NormalizationStatusNormalized,
		IngestionChannel:    domain.IngestionFile,
		ApplicationID:       "550e8400-e29b-41d4-a716-446655440000",
		SourceID:            "660e8400-e29b-41d4-a716-446655440001",
		EnvironmentID:       "770e8400-e29b-41d4-a716-446655440002",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if event.Severity != domain.SeverityDebug {
		t.Fatalf("Severity = %q, want %q", event.Severity, domain.SeverityDebug)
	}
}

func basicSetupFixture() ports.BasicSystemSetup {
	return ports.BasicSystemSetup{
		Application: domain.MonitoredApplication{ID: "550e8400-e29b-41d4-a716-446655440010"},
		Environment: domain.Environment{ID: "770e8400-e29b-41d4-a716-446655440012"},
		Source:      domain.EventSource{ID: "660e8400-e29b-41d4-a716-446655440011"},
	}
}
