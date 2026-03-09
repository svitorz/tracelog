package services

import (
	"context"
	"fmt"
	"strings"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// InitializeSystemInput defines the minimum local setup required for the MVP.
type InitializeSystemInput struct {
	ApplicationName        string
	ApplicationServiceKey  string
	ApplicationDescription string
	EnvironmentCode        string
	EnvironmentName        string
	EnvironmentDescription string
	SourceName             string
	SourceType             string
	SourceDescription      string
}

// InitializeSystemUseCase stores the minimum setup required to run the local MVP.
type InitializeSystemUseCase struct {
	repo ports.SystemSetupRepository
}

func NewInitializeSystemUseCase(repo ports.SystemSetupRepository) *InitializeSystemUseCase {
	return &InitializeSystemUseCase{repo: repo}
}

func (uc *InitializeSystemUseCase) Execute(ctx context.Context, input InitializeSystemInput) (*ports.BasicSystemSetup, error) {
	if uc == nil || uc.repo == nil {
		return nil, fmt.Errorf("system setup repository is required")
	}

	setup, err := normalizeBasicSetupInput(input)
	if err != nil {
		return nil, err
	}

	saved, err := uc.repo.SaveBasicSetup(ctx, setup)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize system setup: %w", err)
	}

	return saved, nil
}

// EnsureSystemSetupUseCase verifies that the minimum local setup exists before startup.
type EnsureSystemSetupUseCase struct {
	repo ports.SystemSetupRepository
}

func NewEnsureSystemSetupUseCase(repo ports.SystemSetupRepository) *EnsureSystemSetupUseCase {
	return &EnsureSystemSetupUseCase{repo: repo}
}

func (uc *EnsureSystemSetupUseCase) Execute(ctx context.Context) error {
	if uc == nil || uc.repo == nil {
		return fmt.Errorf("system setup repository is required")
	}

	ok, err := uc.repo.HasBasicSetup(ctx)
	if err != nil {
		return fmt.Errorf("failed to verify system setup: %w", err)
	}
	if !ok {
		return fmt.Errorf("basic system setup is missing; run `tracelog init` before `tracelog serve`")
	}

	return nil
}

func normalizeBasicSetupInput(input InitializeSystemInput) (ports.BasicSystemSetup, error) {
	setup := ports.BasicSystemSetup{
		Application: domain.MonitoredApplication{
			Name:        strings.TrimSpace(input.ApplicationName),
			ServiceKey:  strings.TrimSpace(input.ApplicationServiceKey),
			Description: strings.TrimSpace(input.ApplicationDescription),
		},
		Environment: domain.Environment{
			Code:        strings.TrimSpace(input.EnvironmentCode),
			Name:        strings.TrimSpace(input.EnvironmentName),
			Description: strings.TrimSpace(input.EnvironmentDescription),
		},
		Source: domain.EventSource{
			Name:        strings.TrimSpace(input.SourceName),
			SourceType:  strings.TrimSpace(input.SourceType),
			Description: strings.TrimSpace(input.SourceDescription),
		},
	}

	switch {
	case setup.Application.Name == "":
		return ports.BasicSystemSetup{}, fmt.Errorf("application name is required")
	case setup.Application.ServiceKey == "":
		return ports.BasicSystemSetup{}, fmt.Errorf("application service key is required")
	case setup.Environment.Code == "":
		return ports.BasicSystemSetup{}, fmt.Errorf("environment code is required")
	case setup.Environment.Name == "":
		return ports.BasicSystemSetup{}, fmt.Errorf("environment name is required")
	case setup.Source.Name == "":
		return ports.BasicSystemSetup{}, fmt.Errorf("event source name is required")
	case setup.Source.SourceType == "":
		return ports.BasicSystemSetup{}, fmt.Errorf("event source type is required")
	}

	return setup, nil
}
