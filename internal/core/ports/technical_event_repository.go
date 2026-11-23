package ports

import (
	"context"
	"errors"
	"time"

	"svitorz/tracelog/internal/core/domain"
)

// TechnicalEventFilters defines basic query filters for technical events.
type TechnicalEventFilters struct {
	ApplicationID string
	EnvironmentID string
	Severity      domain.Severity
	OccurredFrom  *time.Time
	OccurredTo    *time.Time
	Limit         int
	// After, when set, keeps only events strictly older than the cursor in the
	// (occurred_at DESC, id DESC) order, for keyset pagination (spec 0005, seção 4.3).
	After *EventCursor
}

// EventCursor is a position in the (occurred_at DESC, id DESC) order of events.
type EventCursor struct {
	OccurredAt time.Time
	ID         string
}

// ErrReferenceNotFound reports that application_id, source_id or environment_id
// does not reference an existing row. Implementations wrap it with the offending field.
var ErrReferenceNotFound = errors.New("referenced entity does not exist")

// ErrNotFound reports that the requested entity does not exist.
var ErrNotFound = errors.New("not found")

// TechnicalEventRepository defines persistence operations for technical events.
type TechnicalEventRepository interface {
	// Save persists event and reports whether a new row was created. If an event with the
	// same (ApplicationID, ExternalID) already exists, event is replaced by the stored one
	// and created is false. Events without ExternalID are never deduplicated.
	Save(ctx context.Context, event *domain.TechnicalEvent) (created bool, err error)
	// FindByFilters returns events ordered by occurred_at DESC, id DESC.
	FindByFilters(ctx context.Context, filters TechnicalEventFilters) ([]domain.TechnicalEvent, error)
	// FindByID returns ErrNotFound (wrapped) when no event has the given id.
	FindByID(ctx context.Context, id string) (domain.TechnicalEvent, error)
}
