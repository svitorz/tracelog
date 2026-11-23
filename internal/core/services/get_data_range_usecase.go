package services

import (
	"context"
	"fmt"
	"time"

	"svitorz/tracelog/internal/core/ports"
)

// DefaultPeriodDays is the length of the overview's suggested period.
const DefaultPeriodDays = 14

// DataRange is where the events are, plus the overview's suggested period: the
// DefaultPeriodDays UTC days ending on the day of the latest error, else of the latest
// event, else today (spec 0005, seção 4.5).
type DataRange struct {
	ports.EventDataRange
	DefaultFrom time.Time
	DefaultTo   time.Time
}

// GetDataRangeUseCase reports the data range and the suggested period.
type GetDataRangeUseCase struct {
	repo ports.EventStatsRepository
	now  func() time.Time
}

// NewGetDataRangeUseCase uses time.Now when now is nil.
func NewGetDataRangeUseCase(repo ports.EventStatsRepository, now func() time.Time) *GetDataRangeUseCase {
	if now == nil {
		now = time.Now
	}
	return &GetDataRangeUseCase{repo: repo, now: now}
}

func (uc *GetDataRangeUseCase) Execute(ctx context.Context) (DataRange, error) {
	stored, err := uc.repo.DataRange(ctx)
	if err != nil {
		return DataRange{}, fmt.Errorf("failed to get data range: %w", err)
	}

	end := uc.now()
	switch {
	case stored.LastErrorAt != nil:
		end = *stored.LastErrorAt
	case stored.LastOccurredAt != nil:
		end = *stored.LastOccurredAt
	}

	to := utcDay(end)
	return DataRange{
		EventDataRange: stored,
		DefaultFrom:    to.AddDate(0, 0, -(DefaultPeriodDays - 1)),
		DefaultTo:      to,
	}, nil
}
