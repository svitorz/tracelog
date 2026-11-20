package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"svitorz/tracelog/internal/core/domain"

	"github.com/spf13/cobra"
)

func TestPrintErrorFrequencyTableWritesEverythingToStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&stdout)
	c.SetErr(&stderr)

	printErrorFrequencyTable(c, []domain.ErrorFrequency{
		{Date: time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC), Errors: 3, Fatal: 1},
		{Date: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC), Errors: 2, Fatal: 0},
	})

	out := stdout.String()
	for _, want := range []string{
		"DATE        ERRORS  FATAL  TOTAL",
		"2026-03-01  3       1      4",
		"2026-03-02  2       0      2",
		"Errors: 5",
		"Fatal: 1",
		"Average (error+fatal) per day with errors: 3.0",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
