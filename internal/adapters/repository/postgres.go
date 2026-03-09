package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"svitorz/tracelog/internal/config"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const defaultPingTimeout = 5 * time.Second

// OpenPostgres abre um pool SQL para PostgreSQL com base na configuração.
func OpenPostgres(cfg config.DBConfig) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir conexão PostgreSQL: %w", err)
	}

	return db, nil
}

// Ping valida conectividade com timeout controlado.
func Ping(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("conexão PostgreSQL nula")
	}

	ctx, cancel := withDefaultTimeout(ctx, defaultPingTimeout)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("erro no ping PostgreSQL: %w", err)
	}

	return nil
}

func withDefaultTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, timeout)
}
