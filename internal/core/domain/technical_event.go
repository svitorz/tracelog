package domain

import "time"

// TechnicalEvent is the central traceability entity for the MVP.
type TechnicalEvent struct {
	ID                  string
	ExternalID          string
	OccurredAt          time.Time
	ReceivedAt          time.Time
	Message             string
	Severity            Severity
	Category            EventCategory
	Payload             map[string]any
	TraceID             string
	CorrelationID       string
	RawLine             string
	NormalizationStatus NormalizationStatus
	ParseError          string
	IngestionChannel    IngestionChannel
	ApplicationID       string
	SourceID            string
	EnvironmentID       string
	CreatedAt           time.Time
}
