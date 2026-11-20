package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/services"
)

// requestTimeout limits how long a single event registration may take (spec 0002, seção 4.1).
const requestTimeout = 10 * time.Second

// EventHandler handles HTTP operations for technical events.
type EventHandler struct {
	registerUseCase *services.RegisterTechnicalEventUseCase
	maxBodyBytes    int64
}

func NewEventHandler(registerUseCase *services.RegisterTechnicalEventUseCase, maxBodyBytes int64) *EventHandler {
	return &EventHandler{
		registerUseCase: registerUseCase,
		maxBodyBytes:    maxBodyBytes,
	}
}

func (h *EventHandler) HandleRegisterEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, h.maxBodyBytes))
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			respondError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("request body exceeds %d bytes", tooLarge.Limit))
			return
		}
		respondError(w, http.StatusBadRequest, "failed to read request body")
		return
	}

	var req RegisterEventRequest
	var decodeError string
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			decodeError = err.Error()
		}
	}

	input := services.RegisterTechnicalEventInput{
		ExternalID:       req.ExternalID,
		OccurredAt:       req.OccurredAt,
		Message:          req.Message,
		Category:         domain.EventCategory(req.Category),
		Payload:          req.Payload,
		TraceID:          req.TraceID,
		CorrelationID:    req.CorrelationID,
		IngestionChannel: domain.IngestionHTTP,
		ApplicationID:    req.ApplicationID,
		SourceID:         req.SourceID,
		EnvironmentID:    req.EnvironmentID,
		RawRequestBody:   string(body),
		DecodeError:      decodeError,
	}

	ctx, cancel := context.WithTimeout(r.Context(), requestTimeout)
	defer cancel()

	event, err := h.registerUseCase.Execute(ctx, input)
	if err != nil {
		respondError(w, http.StatusInternalServerError, err.Error())
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

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, ErrorResponse{Error: message})
}
