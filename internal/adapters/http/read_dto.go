package http

import (
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/services"
)

// DTOs da API de leitura (spec 0005, seção 5.1). Strings opcionais ausentes saem como
// null e horários sempre em UTC.

// ReadErrorResponse is the error body of the read API.
type ReadErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
	Field string `json:"field,omitempty"`
}

// EventSummaryResponse is one item of the event list.
type EventSummaryResponse struct {
	ID                  string    `json:"id"`
	ExternalID          *string   `json:"external_id"`
	OccurredAt          time.Time `json:"occurred_at"`
	ReceivedAt          time.Time `json:"received_at"`
	CreatedAt           time.Time `json:"created_at"`
	Severity            string    `json:"severity"`
	Category            string    `json:"category"`
	Message             string    `json:"message"`
	NormalizationStatus string    `json:"normalization_status"`
	IngestionChannel    string    `json:"ingestion_channel"`
	TraceID             *string   `json:"trace_id"`
	CorrelationID       *string   `json:"correlation_id"`
	ApplicationID       string    `json:"application_id"`
	EnvironmentID       string    `json:"environment_id"`
	SourceID            string    `json:"source_id"`
}

// EventDetailResponse is a full event.
type EventDetailResponse struct {
	EventSummaryResponse
	ParseError *string        `json:"parse_error"`
	RawLine    *string        `json:"raw_line"`
	Payload    map[string]any `json:"payload"`
}

// EventListResponse is one page of events.
type EventListResponse struct {
	Items      []EventSummaryResponse `json:"items"`
	NextCursor *string                `json:"next_cursor"`
}

// ErrorFrequencyResponse has one entry per UTC day.
type ErrorFrequencyResponse struct {
	From string              `json:"from"`
	To   string              `json:"to"`
	Days []ErrorFrequencyDay `json:"days"`
}

type ErrorFrequencyDay struct {
	Date   string `json:"date"`
	Errors int64  `json:"errors"`
	Fatal  int64  `json:"fatal"`
	Total  int64  `json:"total"`
}

// SeverityCountsResponse uses a struct instead of a map to keep the severity order.
type SeverityCountsResponse struct {
	Counts struct {
		Debug int64 `json:"debug"`
		Info  int64 `json:"info"`
		Warn  int64 `json:"warn"`
		Error int64 `json:"error"`
		Fatal int64 `json:"fatal"`
	} `json:"counts"`
	Total int64 `json:"total"`
}

type DataRangeResponse struct {
	FirstOccurredAt *time.Time `json:"first_occurred_at"`
	LastOccurredAt  *time.Time `json:"last_occurred_at"`
	LastErrorAt     *time.Time `json:"last_error_at"`
	DefaultFrom     string     `json:"default_from"`
	DefaultTo       string     `json:"default_to"`
}

type ApplicationResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ServiceKey  string    `json:"service_key"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type EnvironmentResponse struct {
	ID          string    `json:"id"`
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type SourceResponse struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	SourceType  string    `json:"source_type"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// ListResponse wraps the reference lists.
type ListResponse[T any] struct {
	Items []T `json:"items"`
}

func nullable(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	v := t.UTC()
	return &v
}

func toEventSummary(e domain.TechnicalEvent) EventSummaryResponse {
	return EventSummaryResponse{
		ID:                  e.ID,
		ExternalID:          nullable(e.ExternalID),
		OccurredAt:          e.OccurredAt.UTC(),
		ReceivedAt:          e.ReceivedAt.UTC(),
		CreatedAt:           e.CreatedAt.UTC(),
		Severity:            string(e.Severity),
		Category:            string(e.Category),
		Message:             e.Message,
		NormalizationStatus: string(e.NormalizationStatus),
		IngestionChannel:    string(e.IngestionChannel),
		TraceID:             nullable(e.TraceID),
		CorrelationID:       nullable(e.CorrelationID),
		ApplicationID:       e.ApplicationID,
		EnvironmentID:       e.EnvironmentID,
		SourceID:            e.SourceID,
	}
}

func toEventDetail(e domain.TechnicalEvent) EventDetailResponse {
	payload := e.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	return EventDetailResponse{
		EventSummaryResponse: toEventSummary(e),
		ParseError:           nullable(e.ParseError),
		RawLine:              nullable(e.RawLine),
		Payload:              payload,
	}
}

func toErrorFrequency(f services.DailyErrorFrequency) ErrorFrequencyResponse {
	out := ErrorFrequencyResponse{
		From: f.From.Format(time.DateOnly),
		To:   f.To.Format(time.DateOnly),
		Days: make([]ErrorFrequencyDay, 0, len(f.Days)),
	}
	for _, d := range f.Days {
		out.Days = append(out.Days, ErrorFrequencyDay{
			Date:   d.Date.Format(time.DateOnly),
			Errors: d.Errors,
			Fatal:  d.Fatal,
			Total:  d.Total(),
		})
	}
	return out
}

func toSeverityCounts(c services.SeverityCounts) SeverityCountsResponse {
	var out SeverityCountsResponse
	out.Counts.Debug = c.Counts[domain.SeverityDebug]
	out.Counts.Info = c.Counts[domain.SeverityInfo]
	out.Counts.Warn = c.Counts[domain.SeverityWarn]
	out.Counts.Error = c.Counts[domain.SeverityError]
	out.Counts.Fatal = c.Counts[domain.SeverityFatal]
	out.Total = c.Total
	return out
}

func toDataRange(r services.DataRange) DataRangeResponse {
	return DataRangeResponse{
		FirstOccurredAt: utcPtr(r.FirstOccurredAt),
		LastOccurredAt:  utcPtr(r.LastOccurredAt),
		LastErrorAt:     utcPtr(r.LastErrorAt),
		DefaultFrom:     r.DefaultFrom.Format(time.DateOnly),
		DefaultTo:       r.DefaultTo.Format(time.DateOnly),
	}
}

func mapItems[T, R any](items []T, convert func(T) R) ListResponse[R] {
	out := ListResponse[R]{Items: make([]R, 0, len(items))}
	for _, item := range items {
		out.Items = append(out.Items, convert(item))
	}
	return out
}

func toApplication(a domain.MonitoredApplication) ApplicationResponse {
	return ApplicationResponse{ID: a.ID, Name: a.Name, ServiceKey: a.ServiceKey, Description: nullable(a.Description), CreatedAt: a.CreatedAt.UTC()}
}

func toEnvironment(e domain.Environment) EnvironmentResponse {
	return EnvironmentResponse{ID: e.ID, Code: e.Code, Name: e.Name, Description: nullable(e.Description), CreatedAt: e.CreatedAt.UTC()}
}

func toSource(s domain.EventSource) SourceResponse {
	return SourceResponse{ID: s.ID, Name: s.Name, SourceType: s.SourceType, Description: nullable(s.Description), CreatedAt: s.CreatedAt.UTC()}
}
