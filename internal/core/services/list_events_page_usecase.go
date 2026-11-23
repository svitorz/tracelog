package services

import (
	"context"
	"fmt"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// Page size limits of the HTTP event list (spec 0005, seção 4.3).
const (
	DefaultPageLimit = 50
	MaxPageLimit     = 200
)

// ListEventsPageInput holds the filters of one page of the event list. A nil Limit
// means the default; any explicit value must be within 1..MaxPageLimit.
type ListEventsPageInput struct {
	EventScope
	Severity domain.Severity
	Limit    *int
	After    *ports.EventCursor
}

// EventsPage is one page of events, newest first. Next is nil on the last page.
type EventsPage struct {
	Items []domain.TechnicalEvent
	Next  *ports.EventCursor
}

// ListEventsPageUseCase serves the paginated, strictly validated event list.
type ListEventsPageUseCase struct {
	repo ports.TechnicalEventRepository
}

func NewListEventsPageUseCase(repo ports.TechnicalEventRepository) *ListEventsPageUseCase {
	return &ListEventsPageUseCase{repo: repo}
}

func (uc *ListEventsPageUseCase) Execute(ctx context.Context, input ListEventsPageInput) (EventsPage, error) {
	scope, err := input.EventScope.normalized()
	if err != nil {
		return EventsPage{}, err
	}

	severity := domain.NormalizeSeverity(input.Severity)
	if severity != "" && !isValidSeverity(severity) {
		return EventsPage{}, invalidField("severity", "severity must be one of debug, info, warn, error, fatal")
	}

	limit := DefaultPageLimit
	if input.Limit != nil {
		limit = *input.Limit
		if limit < 1 || limit > MaxPageLimit {
			return EventsPage{}, invalidField("limit", "limit must be an integer between 1 and %d", MaxPageLimit)
		}
	}

	// One extra row tells whether another page exists without a COUNT.
	events, err := uc.repo.FindByFilters(ctx, ports.TechnicalEventFilters{
		ApplicationID: scope.ApplicationID,
		EnvironmentID: scope.EnvironmentID,
		Severity:      severity,
		OccurredFrom:  scope.OccurredFrom,
		OccurredTo:    scope.OccurredTo,
		Limit:         limit + 1,
		After:         input.After,
	})
	if err != nil {
		return EventsPage{}, fmt.Errorf("failed to list technical events: %w", err)
	}

	page := EventsPage{Items: events}
	if len(events) > limit {
		page.Items = events[:limit]
		last := page.Items[limit-1]
		page.Next = &ports.EventCursor{OccurredAt: last.OccurredAt, ID: last.ID}
	}

	return page, nil
}
