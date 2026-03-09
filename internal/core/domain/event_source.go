package domain

import "time"

// EventSource represents a technical origin of events (e.g., API, worker, scheduler).
type EventSource struct {
	ID          string
	Name        string
	SourceType  string
	Description string
	CreatedAt   time.Time
}
