package services

import (
	"context"
	"fmt"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// AllSeverities lists every severity, from least to most severe.
var AllSeverities = []domain.Severity{
	domain.SeverityDebug,
	domain.SeverityInfo,
	domain.SeverityWarn,
	domain.SeverityError,
	domain.SeverityFatal,
}

// SeverityCounts always has one entry per severity in AllSeverities.
type SeverityCounts struct {
	Counts map[domain.Severity]int64
	Total  int64
}

// GetSeverityCountsUseCase counts events per severity in a scope.
type GetSeverityCountsUseCase struct {
	repo ports.EventStatsRepository
}

func NewGetSeverityCountsUseCase(repo ports.EventStatsRepository) *GetSeverityCountsUseCase {
	return &GetSeverityCountsUseCase{repo: repo}
}

func (uc *GetSeverityCountsUseCase) Execute(ctx context.Context, input EventScope) (SeverityCounts, error) {
	scope, err := input.normalized()
	if err != nil {
		return SeverityCounts{}, err
	}

	stored, err := uc.repo.CountBySeverity(ctx, ports.EventStatsFilters{
		ApplicationID: scope.ApplicationID,
		EnvironmentID: scope.EnvironmentID,
		OccurredFrom:  scope.OccurredFrom,
		OccurredTo:    scope.OccurredTo,
	})
	if err != nil {
		return SeverityCounts{}, fmt.Errorf("failed to count events by severity: %w", err)
	}

	result := SeverityCounts{Counts: make(map[domain.Severity]int64, len(AllSeverities))}
	for _, severity := range AllSeverities {
		result.Counts[severity] = stored[severity]
		result.Total += stored[severity]
	}

	return result, nil
}
