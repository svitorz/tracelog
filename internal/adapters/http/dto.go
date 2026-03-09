package http

import (
	"time"
)

// RegisterEventRequest is the HTTP DTO for registering technical events.
type RegisterEventRequest struct {
	ExternalID    string         `json:"external_id"`
	OccurredAt    string         `json:"occurred_at"`
	Message       string         `json:"message"`
	Category      string         `json:"category,omitempty"`
	Payload       map[string]any `json:"payload,omitempty"`
	TraceID       string         `json:"trace_id,omitempty"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	ApplicationID string         `json:"application_id"`
	SourceID      string         `json:"source_id"`
	EnvironmentID string         `json:"environment_id"`
}

// RegisterEventResponse is the HTTP DTO for successful event registration.
type RegisterEventResponse struct {
	ID         string    `json:"id"`
	ExternalID string    `json:"external_id"`
	ReceivedAt time.Time `json:"received_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// ErrorResponse is the standard HTTP error response.
type ErrorResponse struct {
	Error string `json:"error"`
}
