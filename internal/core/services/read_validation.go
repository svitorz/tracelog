package services

import (
	"fmt"
	"strings"
	"time"
)

// ValidationError reports invalid input to a read use case. Field is the name of the
// offending parameter as clients send it (spec 0005, seção 5.2); Message is safe to
// show to the client.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

func invalidField(field, format string, args ...any) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

// IsUUID reports whether value has the 8-4-4-4-12 hexadecimal form PostgreSQL's uuid
// type accepts, in any version.
func IsUUID(value string) bool {
	return isUUIDLike(value)
}

// EventScope is the filter set shared by the event list and the aggregates.
type EventScope struct {
	ApplicationID string
	EnvironmentID string
	OccurredFrom  *time.Time
	OccurredTo    *time.Time
}

func (s EventScope) normalized() (EventScope, error) {
	s.ApplicationID = strings.TrimSpace(s.ApplicationID)
	s.EnvironmentID = strings.TrimSpace(s.EnvironmentID)

	if s.ApplicationID != "" && !IsUUID(s.ApplicationID) {
		return s, invalidField("application_id", "application_id must be a UUID")
	}
	if s.EnvironmentID != "" && !IsUUID(s.EnvironmentID) {
		return s, invalidField("environment_id", "environment_id must be a UUID")
	}
	if s.OccurredFrom != nil && s.OccurredTo != nil && s.OccurredFrom.After(*s.OccurredTo) {
		return s, invalidField("from", "from must not be after to")
	}

	return s, nil
}
