package services

import (
	"context"
	"errors"
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

	result, err := uc.Execute(context.Background(), RegisterTechnicalEventInput{
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

	if result.Event.Severity != domain.SeverityDebug {
		t.Fatalf("Severity = %q, want %q", result.Event.Severity, domain.SeverityDebug)
	}
}

func basicSetupFixture() ports.BasicSystemSetup {
	return ports.BasicSystemSetup{
		Application: domain.MonitoredApplication{ID: "550e8400-e29b-41d4-a716-446655440010"},
		Environment: domain.Environment{ID: "770e8400-e29b-41d4-a716-446655440012"},
		Source:      domain.EventSource{ID: "660e8400-e29b-41d4-a716-446655440011"},
	}
}

func TestImportLogFilesUseCaseSkipsBlankLines(t *testing.T) {
	repo := &stubTechnicalEventRepository{}
	registerUC := NewRegisterTechnicalEventUseCase(repo, &stubSystemSetupRepository{saved: basicSetupFixture()})
	importUC := NewImportLogFilesUseCase(registerUC)
	importUC.openFile = func(string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("first line\n\n   \t\nsecond line\n")), nil
	}

	result, err := importUC.Execute(context.Background(), ImportLogFilesInput{Files: []string{"app.log"}})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if result.LinesRead != 4 || result.BlankSkipped != 2 || result.Persisted != 2 {
		t.Fatalf("LinesRead=%d BlankSkipped=%d Persisted=%d, want 4, 2, 2", result.LinesRead, result.BlankSkipped, result.Persisted)
	}
	if repo.saves != 2 {
		t.Fatalf("repo.Save calls = %d, want 2 (blank lines are not stored)", repo.saves)
	}
}

func TestImportLogFilesUseCaseReportsFailedLines(t *testing.T) {
	repo := &stubTechnicalEventRepository{err: errors.New("connection refused")}
	registerUC := NewRegisterTechnicalEventUseCase(repo, &stubSystemSetupRepository{saved: basicSetupFixture()})
	importUC := NewImportLogFilesUseCase(registerUC)
	importUC.openFile = func(string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(strings.Repeat("some line\n", 12))), nil
	}

	result, err := importUC.Execute(context.Background(), ImportLogFilesInput{Files: []string{"app.log"}})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if result.PersistenceError != 12 {
		t.Fatalf("PersistenceError = %d, want 12", result.PersistenceError)
	}
	if len(result.Errors) != maxReportedImportErrors {
		t.Fatalf("len(Errors) = %d, want %d", len(result.Errors), maxReportedImportErrors)
	}
	first := result.Errors[0]
	if first.File != "app.log" || first.Line != 1 || !strings.Contains(first.Reason, "connection refused") {
		t.Fatalf("Errors[0] = %+v, want app.log line 1 with the repository error", first)
	}
	if result.Errors[9].Line != 10 {
		t.Fatalf("Errors[9].Line = %d, want 10", result.Errors[9].Line)
	}
}

// dedupRepository mimics the per-application deduplication of the PostgreSQL adapter.
type dedupRepository struct {
	seen map[string]bool
}

func (r *dedupRepository) Save(_ context.Context, event *domain.TechnicalEvent) (bool, error) {
	key := event.ApplicationID + "|" + event.ExternalID
	if event.ExternalID != "" && r.seen[key] {
		return false, nil
	}
	r.seen[key] = true
	return true, nil
}

func (r *dedupRepository) FindByFilters(context.Context, ports.TechnicalEventFilters) ([]domain.TechnicalEvent, error) {
	return nil, nil
}

func TestImportLogFilesUseCaseReimportDoesNotDuplicate(t *testing.T) {
	repo := &dedupRepository{seen: map[string]bool{}}
	registerUC := NewRegisterTechnicalEventUseCase(repo, &stubSystemSetupRepository{saved: basicSetupFixture()})
	importUC := NewImportLogFilesUseCase(registerUC)
	content := "2026-03-09T12:00:00Z INFO started\n\n2026-03-09T12:00:01Z ERROR failed\n"
	importUC.openFile = func(string) (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(content)), nil
	}

	first, err := importUC.Execute(context.Background(), ImportLogFilesInput{Files: []string{"logs/app.log"}})
	if err != nil {
		t.Fatalf("first Execute() error = %v", err)
	}
	// Same file name from another directory, as when importing from a container mount.
	second, err := importUC.Execute(context.Background(), ImportLogFilesInput{Files: []string{"/examples/app.log"}})
	if err != nil {
		t.Fatalf("second Execute() error = %v", err)
	}

	if first.Persisted != 2 || first.Duplicates != 0 {
		t.Fatalf("first import Persisted=%d Duplicates=%d, want 2 and 0", first.Persisted, first.Duplicates)
	}
	if second.Persisted != 0 || second.Duplicates != 2 || second.BlankSkipped != 1 {
		t.Fatalf("second import Persisted=%d Duplicates=%d BlankSkipped=%d, want 0, 2, 1", second.Persisted, second.Duplicates, second.BlankSkipped)
	}
}

func TestImportedLineExternalID(t *testing.T) {
	base := importedLineExternalID("logs/app.log", 7, "line content")

	if !strings.HasPrefix(base, "file:") || len(base) != len("file:")+32 {
		t.Fatalf("importedLineExternalID() = %q, want file: + 32 hex chars", base)
	}
	if got := importedLineExternalID("/other/dir/app.log", 7, "line content"); got != base {
		t.Fatalf("same base name in another directory = %q, want %q", got, base)
	}
	for name, got := range map[string]string{
		"other line number": importedLineExternalID("logs/app.log", 8, "line content"),
		"other content":     importedLineExternalID("logs/app.log", 7, "line content!"),
		"other file name":   importedLineExternalID("logs/worker.log", 7, "line content"),
	} {
		if got == base {
			t.Errorf("%s produced the same external_id %q", name, got)
		}
	}
}
