package repository

import (
	"context"
	"database/sql"
	"fmt"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// PostgresSystemSetupRepository persists the minimum system setup for the local MVP.
type PostgresSystemSetupRepository struct {
	db *sql.DB
}

func NewPostgresSystemSetupRepository(db *sql.DB) *PostgresSystemSetupRepository {
	return &PostgresSystemSetupRepository{db: db}
}

func (r *PostgresSystemSetupRepository) SaveBasicSetup(ctx context.Context, setup ports.BasicSystemSetup) (*ports.BasicSystemSetup, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin setup transaction: %w", err)
	}
	defer tx.Rollback()

	saved := &ports.BasicSystemSetup{}

	saved.Environment, err = upsertEnvironment(ctx, tx, setup.Environment)
	if err != nil {
		return nil, err
	}

	saved.Application, err = upsertApplication(ctx, tx, setup.Application)
	if err != nil {
		return nil, err
	}

	saved.Source, err = upsertEventSource(ctx, tx, setup.Source)
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit setup transaction: %w", err)
	}

	return saved, nil
}

func (r *PostgresSystemSetupRepository) HasBasicSetup(ctx context.Context) (bool, error) {
	if r.db == nil {
		return false, fmt.Errorf("database connection is nil")
	}

	query := `
		SELECT
			EXISTS (SELECT 1 FROM environment) AND
			EXISTS (SELECT 1 FROM monitored_application) AND
			EXISTS (SELECT 1 FROM event_source)
	`

	var ok bool
	if err := r.db.QueryRowContext(ctx, query).Scan(&ok); err != nil {
		return false, fmt.Errorf("failed to verify basic system setup: %w", err)
	}

	return ok, nil
}

func (r *PostgresSystemSetupRepository) LoadBasicSetup(ctx context.Context) (*ports.BasicSystemSetup, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `
		SELECT
			(SELECT id FROM monitored_application ORDER BY created_at ASC LIMIT 1) AS application_id,
			(SELECT name FROM monitored_application ORDER BY created_at ASC LIMIT 1) AS application_name,
			(SELECT service_key FROM monitored_application ORDER BY created_at ASC LIMIT 1) AS application_service_key,
			(SELECT COALESCE(description, '') FROM monitored_application ORDER BY created_at ASC LIMIT 1) AS application_description,
			(SELECT created_at FROM monitored_application ORDER BY created_at ASC LIMIT 1) AS application_created_at,
			(SELECT id FROM environment ORDER BY created_at ASC LIMIT 1) AS environment_id,
			(SELECT code FROM environment ORDER BY created_at ASC LIMIT 1) AS environment_code,
			(SELECT name FROM environment ORDER BY created_at ASC LIMIT 1) AS environment_name,
			(SELECT COALESCE(description, '') FROM environment ORDER BY created_at ASC LIMIT 1) AS environment_description,
			(SELECT created_at FROM environment ORDER BY created_at ASC LIMIT 1) AS environment_created_at,
			(SELECT id FROM event_source ORDER BY created_at ASC LIMIT 1) AS source_id,
			(SELECT name FROM event_source ORDER BY created_at ASC LIMIT 1) AS source_name,
			(SELECT source_type FROM event_source ORDER BY created_at ASC LIMIT 1) AS source_type,
			(SELECT COALESCE(description, '') FROM event_source ORDER BY created_at ASC LIMIT 1) AS source_description,
			(SELECT created_at FROM event_source ORDER BY created_at ASC LIMIT 1) AS source_created_at
	`

	var setup ports.BasicSystemSetup
	if err := r.db.QueryRowContext(ctx, query).Scan(
		&setup.Application.ID,
		&setup.Application.Name,
		&setup.Application.ServiceKey,
		&setup.Application.Description,
		&setup.Application.CreatedAt,
		&setup.Environment.ID,
		&setup.Environment.Code,
		&setup.Environment.Name,
		&setup.Environment.Description,
		&setup.Environment.CreatedAt,
		&setup.Source.ID,
		&setup.Source.Name,
		&setup.Source.SourceType,
		&setup.Source.Description,
		&setup.Source.CreatedAt,
	); err != nil {
		return nil, fmt.Errorf("failed to load basic system setup: %w", err)
	}

	if setup.Application.ID == "" || setup.Environment.ID == "" || setup.Source.ID == "" {
		return nil, fmt.Errorf("basic system setup is incomplete")
	}

	return &setup, nil
}

func upsertEnvironment(ctx context.Context, tx *sql.Tx, env domain.Environment) (domain.Environment, error) {
	query := `
		INSERT INTO environment (code, name, description)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (code) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description
		RETURNING id, code, name, COALESCE(description, ''), created_at
	`

	if err := tx.QueryRowContext(ctx, query, env.Code, env.Name, env.Description).
		Scan(&env.ID, &env.Code, &env.Name, &env.Description, &env.CreatedAt); err != nil {
		return domain.Environment{}, fmt.Errorf("failed to save environment: %w", err)
	}

	return env, nil
}

func upsertApplication(ctx context.Context, tx *sql.Tx, app domain.MonitoredApplication) (domain.MonitoredApplication, error) {
	query := `
		INSERT INTO monitored_application (name, service_key, description)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (service_key) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description
		RETURNING id, name, service_key, COALESCE(description, ''), created_at
	`

	if err := tx.QueryRowContext(ctx, query, app.Name, app.ServiceKey, app.Description).
		Scan(&app.ID, &app.Name, &app.ServiceKey, &app.Description, &app.CreatedAt); err != nil {
		return domain.MonitoredApplication{}, fmt.Errorf("failed to save monitored application: %w", err)
	}

	return app, nil
}

func upsertEventSource(ctx context.Context, tx *sql.Tx, source domain.EventSource) (domain.EventSource, error) {
	query := `
		INSERT INTO event_source (name, source_type, description)
		VALUES ($1, $2, NULLIF($3, ''))
		ON CONFLICT (name, source_type) DO UPDATE SET
			description = EXCLUDED.description
		RETURNING id, name, source_type, COALESCE(description, ''), created_at
	`

	if err := tx.QueryRowContext(ctx, query, source.Name, source.SourceType, source.Description).
		Scan(&source.ID, &source.Name, &source.SourceType, &source.Description, &source.CreatedAt); err != nil {
		return domain.EventSource{}, fmt.Errorf("failed to save event source: %w", err)
	}

	return source, nil
}

var _ ports.SystemSetupRepository = (*PostgresSystemSetupRepository)(nil)
