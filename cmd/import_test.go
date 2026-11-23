package cmd

import (
	"bytes"
	"strings"
	"testing"

	"svitorz/tracelog/internal/core/services"

	"github.com/spf13/cobra"
)

func TestPrintImportLogsSummaryListsFailedLines(t *testing.T) {
	var stdout, stderr bytes.Buffer
	c := &cobra.Command{}
	c.SetOut(&stdout)
	c.SetErr(&stderr)

	printImportLogsSummary(c, &services.ImportLogFilesResult{
		LinesRead:        14,
		Persisted:        2,
		PersistenceError: 12,
		Errors: []services.ImportLineError{
			{File: "app.log", Line: 3, Reason: "connection refused"},
			{File: "app.log", Line: 4, Reason: "connection refused"},
		},
	})

	out := stdout.String()
	for _, want := range []string{"Persistence errors: 12", "  app.log:3: connection refused", "  ... and 10 more"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}
