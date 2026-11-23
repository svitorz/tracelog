package config

import (
	"os"

	"github.com/jackc/pgx/v5/pgconn"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadUsesDefaultsAndRequiredDatabaseSettings(t *testing.T) {
	withTempWorkingDir(t)
	withEnv(t, map[string]string{
		"APP_NAME":            "",
		"APP_ENV":             "",
		"HTTP_PORT":           "",
		"DB_HOST":             "",
		"DB_PORT":             "",
		"DB_NAME":             "tracelog",
		"HTTP_MAX_BODY_BYTES": "",
		"DB_USER":             "postgres",
		"DB_PASSWORD":         "postgres",
		"DB_SSLMODE":          "",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error = %v", err)
	}

	if cfg.App.Name != defaultAppName {
		t.Fatalf("App.Name = %q, want %q", cfg.App.Name, defaultAppName)
	}
	if cfg.App.Env != defaultAppEnv {
		t.Fatalf("App.Env = %q, want %q", cfg.App.Env, defaultAppEnv)
	}
	if cfg.HTTP.Port != defaultHTTPPort {
		t.Fatalf("HTTP.Port = %d, want %d", cfg.HTTP.Port, defaultHTTPPort)
	}
	if cfg.DB.Host != defaultDBHost {
		t.Fatalf("DB.Host = %q, want %q", cfg.DB.Host, defaultDBHost)
	}
	if cfg.DB.Port != defaultDBPort {
		t.Fatalf("DB.Port = %d, want %d", cfg.DB.Port, defaultDBPort)
	}
	if cfg.HTTP.MaxBodyBytes != defaultHTTPMaxBodyBytes {
		t.Fatalf("HTTP.MaxBodyBytes = %d, want %d", cfg.HTTP.MaxBodyBytes, defaultHTTPMaxBodyBytes)
	}
	if cfg.DB.SSLMode != defaultDBSSLMode {
		t.Fatalf("DB.SSLMode = %q, want %q", cfg.DB.SSLMode, defaultDBSSLMode)
	}
}

func TestLoadReturnsClearErrorForMissingRequiredDatabaseSettings(t *testing.T) {
	withTempWorkingDir(t)
	withEnv(t, map[string]string{
		"APP_NAME":    "",
		"APP_ENV":     "",
		"HTTP_PORT":   "",
		"DB_HOST":     "",
		"DB_PORT":     "",
		"DB_NAME":     "",
		"DB_USER":     "",
		"DB_PASSWORD": "",
		"DB_SSLMODE":  "",
	})

	_, err := Load()
	if err == nil {
		t.Fatal("Load() error = nil, want configuration error")
	}

	message := err.Error()
	for _, want := range []string{"DB_NAME is required", "DB_USER is required", "DB_PASSWORD is required"} {
		if !strings.Contains(message, want) {
			t.Fatalf("Load() error = %q, want substring %q", message, want)
		}
	}
}

func TestLoadReadsDotEnvWithoutOverridingExistingEnvironment(t *testing.T) {
	dir := withTempWorkingDir(t)
	writeFile(t, filepath.Join(dir, ".env"), "APP_NAME=from-dotenv\nDB_NAME=from-dotenv\nDB_USER=from-dotenv\nDB_PASSWORD=from-dotenv\n")

	withEnv(t, map[string]string{
		"APP_NAME":    "from-env",
		"APP_ENV":     "",
		"HTTP_PORT":   "",
		"DB_HOST":     "",
		"DB_PORT":     "",
		"DB_NAME":     "from-env",
		"DB_USER":     "from-env",
		"DB_PASSWORD": "from-env",
		"DB_SSLMODE":  "",
	})

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error = %v", err)
	}

	if cfg.App.Name != "from-env" {
		t.Fatalf("App.Name = %q, want %q", cfg.App.Name, "from-env")
	}
	if cfg.DB.Name != "from-env" {
		t.Fatalf("DB.Name = %q, want %q", cfg.DB.Name, "from-env")
	}
}

func TestLoadRejectsNonPositiveMaxBodyBytes(t *testing.T) {
	for _, value := range []string{"0", "-1"} {
		t.Run(value, func(t *testing.T) {
			withTempWorkingDir(t)
			withEnv(t, map[string]string{
				"HTTP_MAX_BODY_BYTES": value,
				"DB_NAME":             "tracelog",
				"DB_USER":             "postgres",
				"DB_PASSWORD":         "postgres",
			})

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), "HTTP_MAX_BODY_BYTES") {
				t.Fatalf("Load() error = %v, want HTTP_MAX_BODY_BYTES validation error", err)
			}
		})
	}
}

func withTempWorkingDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}

	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", dir, err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("restore working directory error = %v", err)
		}
	})

	return dir
}

func withEnv(t *testing.T, values map[string]string) {
	t.Helper()

	for key, value := range values {
		if err := os.Setenv(key, value); err != nil {
			t.Fatalf("Setenv(%q) error = %v", key, err)
		}

		t.Cleanup(func() {
			if err := os.Unsetenv(key); err != nil {
				t.Fatalf("Unsetenv(%q) error = %v", key, err)
			}
		})
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func TestDSNQuotesSpecialCharacters(t *testing.T) {
	cfg := DBConfig{Host: "db.local", Port: 5432, Name: "trace log", User: "o'neil", Password: `a b'c\d`, SSLMode: "disable"}

	parsed, err := pgconn.ParseConfig(cfg.DSN())
	if err != nil {
		t.Fatalf("ParseConfig(DSN()) error = %v", err)
	}

	if parsed.Password != `a b'c\d` {
		t.Fatalf("Password = %q, want %q", parsed.Password, `a b'c\d`)
	}
	if parsed.User != "o'neil" || parsed.Database != "trace log" || parsed.Host != "db.local" || parsed.Port != 5432 {
		t.Fatalf("parsed = user %q db %q host %q port %d, want the configured values", parsed.User, parsed.Database, parsed.Host, parsed.Port)
	}
}

func TestLoadValidatesHTTPHost(t *testing.T) {
	tests := map[string]bool{
		"":                 true, // padrão 127.0.0.1
		"0.0.0.0":          true,
		"::1":              true,
		"localhost":        true,
		"api.example.test": true,
		"bad host":         false,
		"-bad":             false,
		"http://x":         false,
	}
	for value, ok := range tests {
		t.Run(value, func(t *testing.T) {
			withTempWorkingDir(t)
			withEnv(t, map[string]string{
				"HTTP_HOST":   value,
				"DB_NAME":     "tracelog",
				"DB_USER":     "postgres",
				"DB_PASSWORD": "postgres",
			})

			cfg, err := Load()
			if ok && err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			if !ok && (err == nil || !strings.Contains(err.Error(), "HTTP_HOST")) {
				t.Fatalf("Load() error = %v, want an HTTP_HOST error", err)
			}
			if value == "" && cfg.HTTP.Host != "127.0.0.1" {
				t.Fatalf("default HTTP.Host = %q, want 127.0.0.1", cfg.HTTP.Host)
			}
		})
	}
}
