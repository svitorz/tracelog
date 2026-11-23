package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The versioned dataset must be exactly what the generator produces with the defaults.
// Regenerate with: go run ./tools/loggen -out examples/logs
func TestExamplesMatchGenerator(t *testing.T) {
	files := generate(defaultConfig())

	for name, want := range files {
		got, err := os.ReadFile(filepath.Join("..", "..", "examples", "logs", name))
		if err != nil {
			t.Fatalf("read examples/logs/%s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("examples/logs/%s differs from the generator output; run: go run ./tools/loggen -out examples/logs", name)
		}
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	cfg := defaultConfig()
	first, second := generate(cfg), generate(cfg)
	for name := range first {
		if !bytes.Equal(first[name], second[name]) {
			t.Fatalf("%s differs between two runs with the same seed", name)
		}
	}

	cfg.Seed++
	if bytes.Equal(generate(cfg)["checkout-api.log"], first["checkout-api.log"]) {
		t.Fatal("a different seed produced the same output")
	}
}
