package ports

import (
	"context"
	"errors"
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

// ErrReferenceNotFound reports that application_id, source_id or environment_id
// does not reference an existing row. Implementations wrap it with the offending field.
var ErrReferenceNotFound = errors.New("referenced entity does not exist")

// TechnicalEventRepository defines persistence operations for technical events.
type TechnicalEventRepository interface {
	Save(ctx context.Context, event *domain.TechnicalEvent) error
	FindByFilters(ctx context.Context, filters TechnicalEventFilters) ([]domain.TechnicalEvent, error)
}
