package services

import (
	"context"
	"fmt"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// ListReferenceDataUseCase lists applications, environments and sources.
type ListReferenceDataUseCase struct {
	repo ports.ReferenceDataRepository
}

func NewListReferenceDataUseCase(repo ports.ReferenceDataRepository) *ListReferenceDataUseCase {
	return &ListReferenceDataUseCase{repo: repo}
}

func (uc *ListReferenceDataUseCase) Applications(ctx context.Context) ([]domain.MonitoredApplication, error) {
	items, err := uc.repo.ListApplications(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list applications: %w", err)
	}
	return items, nil
}

func (uc *ListReferenceDataUseCase) Environments(ctx context.Context) ([]domain.Environment, error) {
	items, err := uc.repo.ListEnvironments(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}
	return items, nil
}

func (uc *ListReferenceDataUseCase) Sources(ctx context.Context) ([]domain.EventSource, error) {
	items, err := uc.repo.ListSources(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sources: %w", err)
	}
	return items, nil
}
