package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// GetErrorFrequencyInput defines the input for error frequency queries.
type GetErrorFrequencyInput struct {
	ApplicationID string
	EnvironmentID string
	OccurredFrom  *time.Time
	OccurredTo    *time.Time
}

// GetErrorFrequencyUseCase handles error frequency aggregation by day.
type GetErrorFrequencyUseCase struct {
	repo ports.ErrorFrequencyRepository
}

func NewGetErrorFrequencyUseCase(repo ports.ErrorFrequencyRepository) *GetErrorFrequencyUseCase {
	return &GetErrorFrequencyUseCase{repo: repo}
}

func (uc *GetErrorFrequencyUseCase) Execute(ctx context.Context, input GetErrorFrequencyInput) ([]domain.ErrorFrequency, error) {
	if uc == nil || uc.repo == nil {
		return nil, fmt.Errorf("error frequency repository is required")
	}

	if input.OccurredFrom != nil && input.OccurredTo != nil && input.OccurredFrom.After(*input.OccurredTo) {
		return nil, fmt.Errorf("period is invalid: from cannot be after to")
	}

	filters := ports.ErrorFrequencyFilters{
		ApplicationID: strings.TrimSpace(input.ApplicationID),
		EnvironmentID: strings.TrimSpace(input.EnvironmentID),
		OccurredFrom:  input.OccurredFrom,
		OccurredTo:    input.OccurredTo,
	}

	frequencies, err := uc.repo.GetErrorFrequencyByDay(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to get error frequency: %w", err)
	}

	return frequencies, nil
}
