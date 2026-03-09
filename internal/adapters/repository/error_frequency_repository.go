package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

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

	query := `
		SELECT
			DATE(occurred_at) AS event_date,
			COUNT(*) AS event_count
		FROM technical_event
		WHERE severity = 'error'
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
		var (
			date  time.Time
			count int64
		)

		if err := rows.Scan(&date, &count); err != nil {
			return nil, fmt.Errorf("failed to scan error frequency: %w", err)
		}

		frequencies = append(frequencies, domain.ErrorFrequency{
			Date:  date,
			Count: count,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating error frequency rows: %w", err)
	}

	return frequencies, nil
}

var _ ports.ErrorFrequencyRepository = (*PostgresErrorFrequencyRepository)(nil)
