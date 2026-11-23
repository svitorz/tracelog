package repository

import (
	"testing"

	"svitorz/tracelog/internal/config"
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
