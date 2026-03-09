package domain

import "time"

// MonitoredApplication identifies an app observed by TraceLog.
type MonitoredApplication struct {
	ID          string
	Name        string
	ServiceKey  string
	Description string
	CreatedAt   time.Time
}
