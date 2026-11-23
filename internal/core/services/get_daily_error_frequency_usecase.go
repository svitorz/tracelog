package services

import (
	"context"
	"fmt"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// MaxFrequencyDays bounds the days returned by GetDailyErrorFrequencyUseCase.
const MaxFrequencyDays = 366

// DailyErrorFrequency has one entry per UTC day from From to To, inclusive, with zeros
// for days without error or fatal events.
type DailyErrorFrequency struct {
	From time.Time
	To   time.Time
	Days []domain.ErrorFrequency
}

// GetDailyErrorFrequencyUseCase is the gap-filled error frequency of the HTTP API. The
// CLI keeps GetErrorFrequencyUseCase, which omits empty days (spec 0002, seção 4.6).
type GetDailyErrorFrequencyUseCase struct {
	repo ports.ErrorFrequencyRepository
}

func NewGetDailyErrorFrequencyUseCase(repo ports.ErrorFrequencyRepository) *GetDailyErrorFrequencyUseCase {
	return &GetDailyErrorFrequencyUseCase{repo: repo}
}

func (uc *GetDailyErrorFrequencyUseCase) Execute(ctx context.Context, input EventScope) (DailyErrorFrequency, error) {
	if input.OccurredFrom == nil {
		return DailyErrorFrequency{}, invalidField("from", "from is required")
	}
	if input.OccurredTo == nil {
		return DailyErrorFrequency{}, invalidField("to", "to is required")
	}

	scope, err := input.normalized()
	if err != nil {
		return DailyErrorFrequency{}, err
	}

	first := utcDay(*scope.OccurredFrom)
	last := utcDay(*scope.OccurredTo)
	if days := int(last.Sub(first)/(24*time.Hour)) + 1; days > MaxFrequencyDays {
		return DailyErrorFrequency{}, invalidField("to", "the period must not exceed %d days", MaxFrequencyDays)
	}

	counts, err := uc.repo.GetErrorFrequencyByDay(ctx, ports.ErrorFrequencyFilters{
		ApplicationID: scope.ApplicationID,
		EnvironmentID: scope.EnvironmentID,
		OccurredFrom:  scope.OccurredFrom,
		OccurredTo:    scope.OccurredTo,
	})
	if err != nil {
		return DailyErrorFrequency{}, fmt.Errorf("failed to get error frequency: %w", err)
	}

	byDay := make(map[string]domain.ErrorFrequency, len(counts))
	for _, c := range counts {
		byDay[c.Date.Format(time.DateOnly)] = c
	}

	result := DailyErrorFrequency{From: first, To: last}
	for day := first; !day.After(last); day = day.AddDate(0, 0, 1) {
		entry := byDay[day.Format(time.DateOnly)]
		entry.Date = day
		result.Days = append(result.Days, entry)
	}

	return result, nil
}

// utcDay returns 00:00 UTC of the day t falls on in UTC.
func utcDay(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
