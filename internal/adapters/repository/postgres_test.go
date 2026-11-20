package repository

import (
	"testing"

	"svitorz/tracelog/internal/config"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestOpenPostgresLimitsPoolSize(t *testing.T) {
	// sql.Open não conecta; o teste não precisa de banco.
	db, err := OpenPostgres(config.DBConfig{Host: "127.0.0.1", Port: 5432, Name: "x", User: "x", Password: "x", SSLMode: "disable"})
	if err != nil {
		t.Fatalf("OpenPostgres() error = %v", err)
	}
	defer db.Close()

	if got := db.Stats().MaxOpenConnections; got != maxOpenConns {
		t.Fatalf("MaxOpenConnections = %d, want %d", got, maxOpenConns)
	}
}

func TestImportDSNDisablesSynchronousCommitOnly(t *testing.T) {
	cfg := config.DBConfig{Host: "127.0.0.1", Port: 5432, Name: "x", User: "x", Password: "a b", SSLMode: "disable"}

	imported, err := pgconn.ParseConfig(importDSN(cfg))
	if err != nil {
		t.Fatalf("ParseConfig(importDSN) error = %v", err)
	}
	if got := imported.RuntimeParams["synchronous_commit"]; got != "off" {
		t.Fatalf("import synchronous_commit = %q, want off", got)
	}
	if imported.Password != "a b" {
		t.Fatalf("import password = %q, want the configured one", imported.Password)
	}

	regular, err := pgconn.ParseConfig(cfg.DSN())
	if err != nil {
		t.Fatalf("ParseConfig(DSN) error = %v", err)
	}
	if _, ok := regular.RuntimeParams["synchronous_commit"]; ok {
		t.Fatal("the regular DSN (serve, CLI) must keep the server default synchronous_commit")
	}
}
