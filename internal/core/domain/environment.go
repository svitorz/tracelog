package domain

import "time"

// Environment represents an execution context (e.g., local, dev, staging, prod).
type Environment struct {
	ID          string
	Code        string
	Name        string
	Description string
	CreatedAt   time.Time
}
