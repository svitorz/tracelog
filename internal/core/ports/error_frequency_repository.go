package ports

import (
	"context"
	"time"

	"svitorz/tracelog/internal/core/domain"
)

// ErrorFrequencyFilters defines query parameters for error frequency aggregation.
type ErrorFrequencyFilters struct {
	ApplicationID string
	EnvironmentID string
	OccurredFrom  *time.Time
	OccurredTo    *time.Time
}

// ErrorFrequencyRepository provides aggregation operations for error metrics.
type ErrorFrequencyRepository interface {
	GetErrorFrequencyByDay(ctx context.Context, filters ErrorFrequencyFilters) ([]domain.ErrorFrequency, error)
}
