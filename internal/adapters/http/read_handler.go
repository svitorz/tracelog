package http

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
	"svitorz/tracelog/internal/core/services"
)

// ReadServices groups the use cases behind the read API (spec 0005).
type ReadServices struct {
	ListEvents     *services.ListEventsPageUseCase
	GetEvent       *services.GetTechnicalEventUseCase
	ErrorFrequency *services.GetDailyErrorFrequencyUseCase
	SeverityCounts *services.GetSeverityCountsUseCase
	DataRange      *services.GetDataRangeUseCase
	Reference      *services.ListReferenceDataUseCase
}

// Error codes of the read API (spec 0005, seção 5.2).
const (
	codeInvalidParameter = "invalid_parameter"
	codeInvalidCursor    = "invalid_cursor"
	codeNotFound         = "not_found"
	codeMethodNotAllowed = "method_not_allowed"
	codeInternal         = "internal"
)

// errInvalidCursor is the only cursor error; the reason stays out of the response.
var errInvalidCursor = errors.New("cursor is malformed")

type readHandler struct {
	svc    *ReadServices
	logger *slog.Logger
}

func registerReadRoutes(mux *http.ServeMux, svc *ReadServices, logger *slog.Logger) {
	h := &readHandler{svc: svc, logger: logger}

	mux.HandleFunc("/api/v1/events", h.route(h.listEvents))
	mux.HandleFunc("/api/v1/events/{id}", h.route(h.getEvent))
	mux.HandleFunc("/api/v1/metrics/error-frequency", h.route(h.errorFrequency))
	mux.HandleFunc("/api/v1/metrics/severity-counts", h.route(h.severityCounts))
	mux.HandleFunc("/api/v1/metrics/data-range", h.route(h.dataRange))
	mux.HandleFunc("/api/v1/applications", h.route(h.applications))
	mux.HandleFunc("/api/v1/environments", h.route(h.environments))
	mux.HandleFunc("/api/v1/sources", h.route(h.sources))
}

// handleAPINotFound answers unknown paths under /api/ in the read API's error format.
func handleAPINotFound(w http.ResponseWriter, _ *http.Request) {
	setAPIHeaders(w)
	respondReadError(w, http.StatusNotFound, codeNotFound, "", "not found")
}

func setAPIHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

// route applies what every read route shares: headers, GET/HEAD only, the request
// timeout and the mapping of use case errors to HTTP.
func (h *readHandler) route(serve func(ctx context.Context, r *http.Request) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setAPIHeaders(w)
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			respondReadError(w, http.StatusMethodNotAllowed, codeMethodNotAllowed, "", "method not allowed")
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
		defer cancel()

		body, err := serve(ctx, r)
		if err != nil {
			h.respondError(w, r, err)
			return
		}
		respondJSON(w, http.StatusOK, body)
	}
}

func (h *readHandler) respondError(w http.ResponseWriter, r *http.Request, err error) {
	var validation *services.ValidationError
	switch {
	case errors.As(err, &validation):
		respondReadError(w, http.StatusBadRequest, codeInvalidParameter, validation.Field, validation.Message)
	case errors.Is(err, errInvalidCursor):
		respondReadError(w, http.StatusBadRequest, codeInvalidCursor, "cursor", errInvalidCursor.Error())
	case errors.Is(err, ports.ErrNotFound):
		respondReadError(w, http.StatusNotFound, codeNotFound, "", "event not found")
	default:
		// The error may carry SQL details: log it, never send it to the client.
		h.logger.ErrorContext(r.Context(), "read request failed", "path", r.URL.Path, "error", err)
		respondReadError(w, http.StatusInternalServerError, codeInternal, "", "internal error")
	}
}

func respondReadError(w http.ResponseWriter, status int, code, field, message string) {
	respondJSON(w, status, ReadErrorResponse{Error: message, Code: code, Field: field})
}

func (h *readHandler) listEvents(ctx context.Context, r *http.Request) (any, error) {
	q, err := parseQuery(r, "severity", "application_id", "environment_id", "from", "to", "limit", "cursor")
	if err != nil {
		return nil, err
	}
	scope, err := parseScope(q)
	if err != nil {
		return nil, err
	}

	input := services.ListEventsPageInput{EventScope: scope, Severity: domain.Severity(q["severity"])}
	if raw, ok := q["limit"]; ok {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			return nil, &services.ValidationError{Field: "limit", Message: fmt.Sprintf("limit must be an integer between 1 and %d", services.MaxPageLimit)}
		}
		input.Limit = &limit
	}
	if raw, ok := q["cursor"]; ok {
		cursor, err := decodeCursor(raw)
		if err != nil {
			return nil, err
		}
		input.After = cursor
	}

	page, err := h.svc.ListEvents.Execute(ctx, input)
	if err != nil {
		return nil, err
	}

	out := EventListResponse{Items: make([]EventSummaryResponse, 0, len(page.Items))}
	for _, e := range page.Items {
		out.Items = append(out.Items, toEventSummary(e))
	}
	if page.Next != nil {
		cursor := encodeCursor(*page.Next)
		out.NextCursor = &cursor
	}
	return out, nil
}

func (h *readHandler) getEvent(ctx context.Context, r *http.Request) (any, error) {
	if _, err := parseQuery(r); err != nil {
		return nil, err
	}
	event, err := h.svc.GetEvent.Execute(ctx, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	return toEventDetail(event), nil
}

func (h *readHandler) errorFrequency(ctx context.Context, r *http.Request) (any, error) {
	q, err := parseQuery(r, "application_id", "environment_id", "from", "to")
	if err != nil {
		return nil, err
	}
	scope, err := parseScope(q)
	if err != nil {
		return nil, err
	}
	frequency, err := h.svc.ErrorFrequency.Execute(ctx, scope)
	if err != nil {
		return nil, err
	}
	return toErrorFrequency(frequency), nil
}

func (h *readHandler) severityCounts(ctx context.Context, r *http.Request) (any, error) {
	q, err := parseQuery(r, "application_id", "environment_id", "from", "to")
	if err != nil {
		return nil, err
	}
	scope, err := parseScope(q)
	if err != nil {
		return nil, err
	}
	counts, err := h.svc.SeverityCounts.Execute(ctx, scope)
	if err != nil {
		return nil, err
	}
	return toSeverityCounts(counts), nil
}

func (h *readHandler) dataRange(ctx context.Context, r *http.Request) (any, error) {
	if _, err := parseQuery(r); err != nil {
		return nil, err
	}
	dr, err := h.svc.DataRange.Execute(ctx)
	if err != nil {
		return nil, err
	}
	return toDataRange(dr), nil
}

func (h *readHandler) applications(ctx context.Context, r *http.Request) (any, error) {
	if _, err := parseQuery(r); err != nil {
		return nil, err
	}
	items, err := h.svc.Reference.Applications(ctx)
	if err != nil {
		return nil, err
	}
	return mapItems(items, toApplication), nil
}

func (h *readHandler) environments(ctx context.Context, r *http.Request) (any, error) {
	if _, err := parseQuery(r); err != nil {
		return nil, err
	}
	items, err := h.svc.Reference.Environments(ctx)
	if err != nil {
		return nil, err
	}
	return mapItems(items, toEnvironment), nil
}

func (h *readHandler) sources(ctx context.Context, r *http.Request) (any, error) {
	if _, err := parseQuery(r); err != nil {
		return nil, err
	}
	items, err := h.svc.Reference.Sources(ctx)
	if err != nil {
		return nil, err
	}
	return mapItems(items, toSource), nil
}

// parseQuery returns the non-empty parameters. An unknown or repeated parameter is a
// 400: a mistyped filter must not silently return unfiltered data (spec 0005, seção 4.2).
func parseQuery(r *http.Request, allowed ...string) (map[string]string, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return nil, &services.ValidationError{Message: "malformed query string"}
	}

	out := make(map[string]string, len(values))
	for name, list := range values {
		if !slices.Contains(allowed, name) {
			return nil, &services.ValidationError{Field: shorten(name), Message: fmt.Sprintf("unknown parameter %q", shorten(name))}
		}
		if len(list) > 1 {
			return nil, &services.ValidationError{Field: name, Message: fmt.Sprintf("parameter %q must not be repeated", name)}
		}
		if value := strings.TrimSpace(list[0]); value != "" {
			out[name] = value
		}
	}
	return out, nil
}

// shorten keeps an echoed parameter name small.
func shorten(name string) string {
	const max = 64
	if len(name) <= max {
		return name
	}
	return name[:max]
}

func parseScope(q map[string]string) (services.EventScope, error) {
	scope := services.EventScope{ApplicationID: q["application_id"], EnvironmentID: q["environment_id"]}

	var err error
	if scope.OccurredFrom, err = parseTimeParam("from", q["from"], false); err != nil {
		return scope, err
	}
	if scope.OccurredTo, err = parseTimeParam("to", q["to"], true); err != nil {
		return scope, err
	}
	return scope, nil
}

// parseTimeParam accepts RFC3339, or YYYY-MM-DD as 00:00 UTC (from) or the whole day
// (to), like the CLI (spec 0002, seção 4.7).
func parseTimeParam(field, raw string, endOfDay bool) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		v := t.UTC()
		return &v, nil
	}
	if t, err := time.Parse(time.DateOnly, raw); err == nil {
		if endOfDay {
			t = t.Add(24*time.Hour - time.Nanosecond)
		}
		return &t, nil
	}
	return nil, &services.ValidationError{Field: field, Message: field + " must be RFC3339 or YYYY-MM-DD"}
}

// cursorPayload is the JSON inside the opaque cursor (spec 0005, seção 4.3).
type cursorPayload struct {
	T  string `json:"t"`
	ID string `json:"id"`
}

func encodeCursor(c ports.EventCursor) string {
	raw, _ := json.Marshal(cursorPayload{T: c.OccurredAt.UTC().Format(time.RFC3339Nano), ID: c.ID})
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodeCursor(value string) (*ports.EventCursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, errInvalidCursor
	}

	var payload cursorPayload
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&payload); err != nil {
		return nil, errInvalidCursor
	}

	t, err := time.Parse(time.RFC3339Nano, payload.T)
	if err != nil || !services.IsUUID(payload.ID) {
		return nil, errInvalidCursor
	}

	return &ports.EventCursor{OccurredAt: t, ID: payload.ID}, nil
}
