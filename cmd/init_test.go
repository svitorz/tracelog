package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// failingReader fails the test if anything reads stdin.
type failingReader struct{ t *testing.T }

func (r failingReader) Read([]byte) (int, error) {
	r.t.Fatal("init --defaults must not read stdin")
	return 0, nil
}

func TestCollectInitInputWithDefaultsDoesNotReadStdin(t *testing.T) {
	c := &cobra.Command{}
	var out bytes.Buffer
	c.SetIn(failingReader{t})
	c.SetOut(&out)
	c.SetErr(&out)

	input := collectInitInput(c, true)

	if input.ApplicationServiceKey != "local-app" || input.EnvironmentCode != "local" || input.SourceType != "http" {
		t.Fatalf("collectInitInput(defaults) = %+v, want the default values", input)
	}
	if strings.Contains(out.String(), "[local-app]:") {
		t.Fatalf("output = %q, want no prompts", out.String())
	}
}

func TestCollectInitInputInteractiveUsesAnswers(t *testing.T) {
	c := &cobra.Command{}
	var out bytes.Buffer
	c.SetIn(strings.NewReader("Billing API\nbilling\n\nprod\n"))
	c.SetOut(&out)
	c.SetErr(&out)

	input := collectInitInput(c, false)

	if input.ApplicationName != "Billing API" || input.ApplicationServiceKey != "billing" {
		t.Fatalf("application = (%q, %q), want the typed answers", input.ApplicationName, input.ApplicationServiceKey)
	}
	if input.ApplicationDescription != "Default monitored application for the local MVP" {
		t.Fatalf("empty answer = %q, want the default", input.ApplicationDescription)
	}
	if input.EnvironmentCode != "prod" || input.SourceType != "http" {
		t.Fatalf("environment code = %q, source type = %q; want prod and the default after EOF", input.EnvironmentCode, input.SourceType)
	}
}
