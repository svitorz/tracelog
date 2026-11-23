package ports

import (
	"context"
	"time"

	"svitorz/tracelog/internal/core/domain"
)

// EventStatsFilters restricts the events counted by EventStatsRepository.
type EventStatsFilters struct {
	ApplicationID string
	EnvironmentID string
	OccurredFrom  *time.Time
	OccurredTo    *time.Time
}

// EventDataRange describes where the stored events are in time. Fields are nil when
// there is no matching event.
type EventDataRange struct {
	FirstOccurredAt *time.Time
	LastOccurredAt  *time.Time
	// LastErrorAt is the most recent occurred_at among error and fatal events.
	LastErrorAt *time.Time
}

// EventStatsRepository provides aggregate reads over technical events.
type EventStatsRepository interface {
	// CountBySeverity returns only the severities that have events.
	CountBySeverity(ctx context.Context, filters EventStatsFilters) (map[domain.Severity]int64, error)
	DataRange(ctx context.Context) (EventDataRange, error)
}
