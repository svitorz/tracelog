package ports

import (
	"context"

	"svitorz/tracelog/internal/core/domain"
)

// ReferenceDataRepository lists the setup entities events point to.
type ReferenceDataRepository interface {
	// ListApplications orders by name.
	ListApplications(ctx context.Context) ([]domain.MonitoredApplication, error)
	// ListEnvironments orders by code.
	ListEnvironments(ctx context.Context) ([]domain.Environment, error)
	// ListSources orders by name.
	ListSources(ctx context.Context) ([]domain.EventSource, error)
}
