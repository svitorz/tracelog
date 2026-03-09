package domain

import "strings"

// Severity represents the severity level of a technical event.
type Severity string

const (
	SeverityDebug Severity = "debug"
	SeverityInfo  Severity = "info"
	SeverityWarn  Severity = "warn"
	SeverityError Severity = "error"
	SeverityFatal Severity = "fatal"
)

// NormalizeSeverity trims and lowercases a severity value for comparisons and persistence.
func NormalizeSeverity(value Severity) Severity {
	return Severity(strings.ToLower(strings.TrimSpace(string(value))))
}

// EventCategory represents the functional category of the event.
type EventCategory string

const (
	CategoryApplication EventCategory = "application"
	CategorySystem      EventCategory = "system"
	CategorySecurity    EventCategory = "security"
	CategoryAudit       EventCategory = "audit"
)

// IngestionChannel represents how the event entered the system.
type IngestionChannel string

const (
	IngestionHTTP IngestionChannel = "http"
	IngestionCLI  IngestionChannel = "cli"
	IngestionFile IngestionChannel = "file"
)
