package repository

import (
	"context"
	"database/sql"
	"fmt"

	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// PostgresEventStatsRepository answers aggregate reads over technical_event.
type PostgresEventStatsRepository struct {
	db *sql.DB
}

func NewPostgresEventStatsRepository(db *sql.DB) *PostgresEventStatsRepository {
	return &PostgresEventStatsRepository{db: db}
}

func (r *PostgresEventStatsRepository) CountBySeverity(ctx context.Context, filters ports.EventStatsFilters) (map[domain.Severity]int64, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT severity, COUNT(*) FROM technical_event WHERE 1=1`
	args := make([]any, 0, 4)

	if filters.ApplicationID != "" {
		args = append(args, filters.ApplicationID)
		query += fmt.Sprintf(" AND application_id = $%d::uuid", len(args))
	}
	if filters.EnvironmentID != "" {
		args = append(args, filters.EnvironmentID)
		query += fmt.Sprintf(" AND environment_id = $%d::uuid", len(args))
	}
	if filters.OccurredFrom != nil {
		args = append(args, *filters.OccurredFrom)
		query += fmt.Sprintf(" AND occurred_at >= $%d", len(args))
	}
	if filters.OccurredTo != nil {
		args = append(args, *filters.OccurredTo)
		query += fmt.Sprintf(" AND occurred_at <= $%d", len(args))
	}
	query += " GROUP BY severity"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to count events by severity: %w", err)
	}
	defer rows.Close()

	counts := map[domain.Severity]int64{}
	for rows.Next() {
		var severity string
		var count int64
		if err := rows.Scan(&severity, &count); err != nil {
			return nil, fmt.Errorf("failed to scan severity count: %w", err)
		}
		counts[domain.Severity(severity)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating severity counts: %w", err)
	}

	return counts, nil
}

func (r *PostgresEventStatsRepository) DataRange(ctx context.Context) (ports.EventDataRange, error) {
	if r.db == nil {
		return ports.EventDataRange{}, fmt.Errorf("database connection is nil")
	}

	// Separate subqueries let min/max walk idx_technical_event_occurred_at instead of
	// scanning the table, which a single aggregate with FILTER would do.
	query := `
		SELECT
			(SELECT MIN(occurred_at) FROM technical_event),
			(SELECT MAX(occurred_at) FROM technical_event),
			(SELECT MAX(occurred_at) FROM technical_event WHERE severity IN ('error', 'fatal'))`

	var first, last, lastError sql.NullTime
	if err := r.db.QueryRowContext(ctx, query).Scan(&first, &last, &lastError); err != nil {
		return ports.EventDataRange{}, fmt.Errorf("failed to get event data range: %w", err)
	}

	return ports.EventDataRange{
		FirstOccurredAt: nullTimePtr(first),
		LastOccurredAt:  nullTimePtr(last),
		LastErrorAt:     nullTimePtr(lastError),
	}, nil
}

func nullTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	return &v.Time
}

var _ ports.EventStatsRepository = (*PostgresEventStatsRepository)(nil)
