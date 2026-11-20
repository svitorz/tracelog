package services

import (
	"testing"

	"svitorz/tracelog/internal/core/domain"
)

// Documenta o categorizador do import (spec 0004, seção 4.4).
func TestDefaultLogLineCategorizer(t *testing.T) {
	tests := []struct {
		message string
		want    domain.EventCategory
		note    string
	}{
		{"unauthorized access attempt from 203.0.113.7", domain.CategorySecurity, "antes: audit"},
		{"login failed for user-0001", domain.CategorySecurity, "antes: audit"},
		{"authentication failure: invalid password", domain.CategorySecurity, "antes: audit"},
		{"invalid token presented", domain.CategorySecurity, ""},
		{"access denied to /admin", domain.CategorySecurity, ""},
		{"user login succeeded", domain.CategoryAudit, ""},
		{"user-0001 logout", domain.CategoryAudit, ""},
		{"order updated by user-0002", domain.CategoryAudit, ""},
		{"database connection timeout", domain.CategorySystem, ""},
		{"3 connections dropped", domain.CategorySystem, ""},
		{"authorization header parsed", domain.CategoryApplication, "antes: audit ('auth' como substring)"},
		{"order 42 shipped", domain.CategoryApplication, ""},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			got, _ := defaultLogLineCategorizer{}.Categorize(parsedLogLine{
				Message:             tt.message,
				Format:              "timestamp_level_message",
				NormalizationStatus: domain.NormalizationStatusNormalized,
				Payload:             map[string]any{},
			})
			if got != tt.want {
				t.Fatalf("Categorize(%q) = %q, want %q %s", tt.message, got, tt.want, tt.note)
			}
		})
	}
}
