package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/services"
)

// EventHandler handles HTTP operations for technical events.
type EventHandler struct {
	registerUseCase *services.RegisterTechnicalEventUseCase
}

func NewEventHandler(registerUseCase *services.RegisterTechnicalEventUseCase) *EventHandler {
	return &EventHandler{
		registerUseCase: registerUseCase,
	}
}

func (h *EventHandler) HandleRegisterEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req RegisterEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON payload: %v", err))
		return
	}

	occurredAt, err := parseOccurredAt(req.OccurredAt)
	if err != nil {
		respondError(w, http.StatusBadRequest, fmt.Sprintf("invalid occurred_at: %v", err))
		return
	}

	input := services.RegisterTechnicalEventInput{
		ExternalID:       req.ExternalID,
		OccurredAt:       occurredAt,
		Message:          req.Message,
		Severity:         domain.Severity(req.Severity),
		Category:         domain.EventCategory(req.Category),
		Payload:          req.Payload,
		TraceID:          req.TraceID,
		CorrelationID:    req.CorrelationID,
		IngestionChannel: domain.IngestionHTTP,
		ApplicationID:    req.ApplicationID,
		SourceID:         req.SourceID,
		EnvironmentID:    req.EnvironmentID,
	}

	event, err := h.registerUseCase.Execute(context.Background(), input)
	if err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	response := RegisterEventResponse{
		ID:         event.ID,
		ExternalID: event.ExternalID,
		ReceivedAt: event.ReceivedAt,
		CreatedAt:  event.CreatedAt,
	}

	respondJSON(w, http.StatusCreated, response)
}

func parseOccurredAt(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, fmt.Errorf("occurred_at is required")
	}

	formats := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		t, err := time.Parse(format, value)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported format (use RFC3339)")
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{Error: message})
}
