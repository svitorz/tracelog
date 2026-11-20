package repository

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

// migrationsLockKey identifies TraceLog's advisory lock; any constant unique to the app works.
const migrationsLockKey = 7_302_026

// MigrationResult reports what Migrate did.
type MigrationResult struct {
	Applied []string
	Total   int
}

// Migrate applies, in lexical order, every *.sql in files that is not yet recorded in
// schema_migrations, recording each one right after it runs (spec 0003, seção 4.1).
//
// Each file manages its own transaction (BEGIN/COMMIT). If the process dies between
// applying a file and recording it, the next run applies it again: migrations must stay
// idempotent (AGENTS.md, seção 5).
func Migrate(ctx context.Context, db *sql.DB, files fs.FS) (MigrationResult, error) {
	names, err := fs.Glob(files, "*.sql")
	if err != nil {
		return MigrationResult{}, fmt.Errorf("failed to list migrations: %w", err)
	}
	sort.Strings(names)

	// A session-level advisory lock needs every statement on the same connection.
	conn, err := db.Conn(ctx)
	if err != nil {
		return MigrationResult{}, fmt.Errorf("failed to get a database connection: %w", err)
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationsLockKey); err != nil {
		return MigrationResult{}, fmt.Errorf("failed to acquire migrations lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationsLockKey)
	}()

	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`); err != nil {
		return MigrationResult{}, fmt.Errorf("failed to create schema_migrations: %w", err)
	}

	applied, err := appliedMigrations(ctx, conn)
	if err != nil {
		return MigrationResult{}, err
	}

	result := MigrationResult{Total: len(names)}
	for _, name := range names {
		version := strings.TrimSuffix(name, ".sql")
		if applied[version] {
			continue
		}

		content, err := fs.ReadFile(files, name)
		if err != nil {
			return result, fmt.Errorf("failed to read migration %s: %w", name, err)
		}

		// Exec without arguments uses the simple protocol, which accepts several statements.
		if _, err := conn.ExecContext(ctx, string(content)); err != nil {
			return result, fmt.Errorf("failed to apply migration %s: %w", version, err)
		}

		if _, err := conn.ExecContext(ctx, `INSERT INTO schema_migrations (version) VALUES ($1)`, version); err != nil {
			return result, fmt.Errorf("failed to record migration %s: %w", version, err)
		}

		result.Applied = append(result.Applied, version)
	}

	return result, nil
}

func appliedMigrations(ctx context.Context, conn *sql.Conn) (map[string]bool, error) {
	rows, err := conn.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema_migrations: %w", err)
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var version string
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan schema_migrations: %w", err)
		}
		applied[version] = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating schema_migrations: %w", err)
	}

	return applied, nil
}
