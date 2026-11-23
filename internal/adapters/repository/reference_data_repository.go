package repository

import (
	"context"
	"database/sql"
	"fmt"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// PostgresReferenceDataRepository lists the setup tables.
type PostgresReferenceDataRepository struct {
	db *sql.DB
}

func NewPostgresReferenceDataRepository(db *sql.DB) *PostgresReferenceDataRepository {
	return &PostgresReferenceDataRepository{db: db}
}

func (r *PostgresReferenceDataRepository) ListApplications(ctx context.Context) ([]domain.MonitoredApplication, error) {
	query := `SELECT id, name, service_key, COALESCE(description, ''), created_at FROM monitored_application ORDER BY name`
	return listRows(ctx, r.db, query, "applications", func(rows *sql.Rows) (domain.MonitoredApplication, error) {
		var a domain.MonitoredApplication
		err := rows.Scan(&a.ID, &a.Name, &a.ServiceKey, &a.Description, &a.CreatedAt)
		return a, err
	})
}

func (r *PostgresReferenceDataRepository) ListEnvironments(ctx context.Context) ([]domain.Environment, error) {
	query := `SELECT id, code, name, COALESCE(description, ''), created_at FROM environment ORDER BY code`
	return listRows(ctx, r.db, query, "environments", func(rows *sql.Rows) (domain.Environment, error) {
		var e domain.Environment
		err := rows.Scan(&e.ID, &e.Code, &e.Name, &e.Description, &e.CreatedAt)
		return e, err
	})
}

func (r *PostgresReferenceDataRepository) ListSources(ctx context.Context) ([]domain.EventSource, error) {
	query := `SELECT id, name, source_type, COALESCE(description, ''), created_at FROM event_source ORDER BY name`
	return listRows(ctx, r.db, query, "sources", func(rows *sql.Rows) (domain.EventSource, error) {
		var s domain.EventSource
		err := rows.Scan(&s.ID, &s.Name, &s.SourceType, &s.Description, &s.CreatedAt)
		return s, err
	})
}

func listRows[T any](ctx context.Context, db *sql.DB, query, what string, scan func(*sql.Rows) (T, error)) ([]T, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query %s: %w", what, err)
	}
	defer rows.Close()

	items := make([]T, 0)
	for rows.Next() {
		item, err := scan(rows)
		if err != nil {
			return nil, fmt.Errorf("failed to scan %s: %w", what, err)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating %s: %w", what, err)
	}

	return items, nil
}

var _ ports.ReferenceDataRepository = (*PostgresReferenceDataRepository)(nil)
