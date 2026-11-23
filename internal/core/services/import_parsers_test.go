package services

import (
	"testing"
	"time"

	"svitorz/tracelog/internal/core/domain"
)

// Documenta qual parser trata cada formato de linha (spec 0004).
func TestImportParseLineChoosesParser(t *testing.T) {
	uc := NewImportLogFilesUseCase(NewRegisterTechnicalEventUseCase(&stubTechnicalEventRepository{}))
	uc.now = func() time.Time { return time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC) }

	tests := []struct {
		line       string
		wantParser string
		wantLevel  any
		wantStatus domain.NormalizationStatus
	}{
		{`{"timestamp":"2026-03-06T02:00:00Z","level":"warn","message":"x"}`, "json", "warn", domain.NormalizationStatusNormalized},
		{`[{"message":"array of objects"}]`, "json", nil, domain.NormalizationStatusPartial},
		{`{"message":"cut in the mid`, "json-fallback", nil, domain.NormalizationStatusPartial},
		{`2026-03-06T02:00:00Z WARN login failed`, "timestamp_level_message", "warn", domain.NormalizationStatusNormalized},
		// Antes ia para o parser JSON, falhava e virava texto puro sem nível.
		{`[06/Mar/2026:02:26:01 +0000] WARN account locked`, "timestamp_level_message", "warn", domain.NormalizationStatusPartial},
		{`gateway started on port 9000`, "plain_text", nil, domain.NormalizationStatusPartial},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			got := uc.parseLine(tt.line)
			if got.Parser != tt.wantParser {
				t.Fatalf("parser = %q, want %q", got.Parser, tt.wantParser)
			}
			if got.Payload["level"] != tt.wantLevel {
				t.Fatalf("level = %v, want %v", got.Payload["level"], tt.wantLevel)
			}
			if got.NormalizationStatus != tt.wantStatus {
				t.Fatalf("status = %q, want %q", got.NormalizationStatus, tt.wantStatus)
			}
		})
	}
}
