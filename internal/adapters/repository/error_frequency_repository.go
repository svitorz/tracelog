package repository

import (
	"context"
	"database/sql"
	"fmt"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// PostgresErrorFrequencyRepository implements error frequency aggregation queries.
type PostgresErrorFrequencyRepository struct {
	db *sql.DB
}

func NewPostgresErrorFrequencyRepository(db *sql.DB) *PostgresErrorFrequencyRepository {
	return &PostgresErrorFrequencyRepository{db: db}
}

func (r *PostgresErrorFrequencyRepository) GetErrorFrequencyByDay(ctx context.Context, filters ports.ErrorFrequencyFilters) ([]domain.ErrorFrequency, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	// Days are UTC regardless of the session TimeZone, matching the UTC --from/--to of the CLI.
	query := `
		SELECT
			(occurred_at AT TIME ZONE 'UTC')::date AS event_date,
			COUNT(*) FILTER (WHERE severity = 'error') AS error_count,
			COUNT(*) FILTER (WHERE severity = 'fatal') AS fatal_count
		FROM technical_event
		WHERE severity IN ('error', 'fatal')
	`

	args := make([]any, 0, 4)
	argPos := 1

	if filters.ApplicationID != "" {
		query += fmt.Sprintf(" AND application_id = $%d::uuid", argPos)
		args = append(args, filters.ApplicationID)
		argPos++
	}

	if filters.EnvironmentID != "" {
		query += fmt.Sprintf(" AND environment_id = $%d::uuid", argPos)
		args = append(args, filters.EnvironmentID)
		argPos++
	}

	if filters.OccurredFrom != nil {
		query += fmt.Sprintf(" AND occurred_at >= $%d", argPos)
		args = append(args, *filters.OccurredFrom)
		argPos++
	}

	if filters.OccurredTo != nil {
		query += fmt.Sprintf(" AND occurred_at <= $%d", argPos)
		args = append(args, *filters.OccurredTo)
		argPos++
	}

	query += " GROUP BY event_date ORDER BY event_date ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query error frequency: %w", err)
	}
	defer rows.Close()

	frequencies := make([]domain.ErrorFrequency, 0)
	for rows.Next() {
		var freq domain.ErrorFrequency
		if err := rows.Scan(&freq.Date, &freq.Errors, &freq.Fatal); err != nil {
			return nil, fmt.Errorf("failed to scan error frequency: %w", err)
		}

		frequencies = append(frequencies, freq)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating error frequency rows: %w", err)
	}

	return frequencies, nil
}

var _ ports.ErrorFrequencyRepository = (*PostgresErrorFrequencyRepository)(nil)
