package cmd

import (
	"testing"
	"time"
)

func TestParsePeriodBounds(t *testing.T) {
	tests := []struct {
		name  string
		parse func(string) (*time.Time, error)
		raw   string
		want  time.Time
	}{
		{"from date starts the day", parsePeriodValue, "2026-03-09", time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC)},
		{"to date ends the day", parsePeriodEnd, "2026-03-09", time.Date(2026, 3, 9, 23, 59, 59, 999999999, time.UTC)},
		{"to RFC3339 is exact", parsePeriodEnd, "2026-03-09T12:00:00Z", time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)},
		{"RFC3339 offset becomes UTC", parsePeriodValue, "2026-03-09T09:00:00-03:00", time.Date(2026, 3, 9, 12, 0, 0, 0, time.UTC)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.parse(tt.raw)
			if err != nil {
				t.Fatalf("parse(%q) error = %v", tt.raw, err)
			}
			if !got.Equal(tt.want) {
				t.Fatalf("parse(%q) = %s, want %s", tt.raw, got, tt.want)
			}
		})
	}

	if got, err := parsePeriodEnd(""); err != nil || got != nil {
		t.Fatalf("parsePeriodEnd(\"\") = (%v, %v), want (nil, nil)", got, err)
	}
	if _, err := parsePeriodEnd("09/03/2026"); err == nil {
		t.Fatal("parsePeriodEnd(09/03/2026) error = nil, want format error")
	}
}
