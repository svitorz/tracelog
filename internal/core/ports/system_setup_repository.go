package ports

import (
	"context"

	"svitorz/tracelog/internal/core/domain"
)

// BasicSystemSetup groups the minimum data required for the local MVP to operate.
type BasicSystemSetup struct {
	Application domain.MonitoredApplication
	Environment domain.Environment
	Source      domain.EventSource
}

// SystemSetupRepository persists and verifies the minimum system configuration.
type SystemSetupRepository interface {
	SaveBasicSetup(ctx context.Context, setup BasicSystemSetup) (*BasicSystemSetup, error)
	HasBasicSetup(ctx context.Context) (bool, error)
}
