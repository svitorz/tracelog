package ports

import (
	"context"
	"time"

	"svitorz/tracelog/internal/core/domain"
)

// TechnicalEventFilters defines basic query filters for technical events.
type TechnicalEventFilters struct {
	ApplicationID string
	EnvironmentID string
	Severity      domain.Severity
	OccurredFrom  *time.Time
	OccurredTo    *time.Time
	Limit         int
}

// TechnicalEventRepository defines persistence operations for technical events.
type TechnicalEventRepository interface {
	Save(ctx context.Context, event *domain.TechnicalEvent) error
	FindByFilters(ctx context.Context, filters TechnicalEventFilters) ([]domain.TechnicalEvent, error)
}
