package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

const defaultListLimit = 50

// ListTechnicalEventsInput defines the available query filters for the CLI list command.
type ListTechnicalEventsInput struct {
	Severity      domain.Severity
	ApplicationID string
	EnvironmentID string
	OccurredFrom  *time.Time
	OccurredTo    *time.Time
	Limit         int
}

// ListTechnicalEventsUseCase handles technical event queries.
type ListTechnicalEventsUseCase struct {
	repo ports.TechnicalEventRepository
}

func NewListTechnicalEventsUseCase(repo ports.TechnicalEventRepository) *ListTechnicalEventsUseCase {
	return &ListTechnicalEventsUseCase{repo: repo}
}

func (uc *ListTechnicalEventsUseCase) Execute(ctx context.Context, input ListTechnicalEventsInput) ([]domain.TechnicalEvent, error) {
	if uc == nil || uc.repo == nil {
		return nil, fmt.Errorf("technical event repository is required")
	}

	input.Severity = domain.NormalizeSeverity(input.Severity)

	if input.Severity != "" && !isValidSeverity(input.Severity) {
		return nil, fmt.Errorf("severity is invalid")
	}

	if input.OccurredFrom != nil && input.OccurredTo != nil && input.OccurredFrom.After(*input.OccurredTo) {
		return nil, fmt.Errorf("period is invalid: from cannot be after to")
	}

	limit := input.Limit
	if limit <= 0 {
		limit = defaultListLimit
	}

	filters := ports.TechnicalEventFilters{
		ApplicationID: strings.TrimSpace(input.ApplicationID),
		EnvironmentID: strings.TrimSpace(input.EnvironmentID),
		Severity:      input.Severity,
		OccurredFrom:  input.OccurredFrom,
		OccurredTo:    input.OccurredTo,
		Limit:         limit,
	}

	events, err := uc.repo.FindByFilters(ctx, filters)
	if err != nil {
		return nil, fmt.Errorf("failed to list technical events: %w", err)
	}

	return events, nil
}
