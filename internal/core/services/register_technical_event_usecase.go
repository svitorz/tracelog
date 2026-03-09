package services

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// RegisterTechnicalEventInput defines the input payload for registering a technical event.
type RegisterTechnicalEventInput struct {
	ExternalID       string
	OccurredAt       time.Time
	Message          string
	Category         domain.EventCategory
	Payload          map[string]any
	TraceID          string
	CorrelationID    string
	IngestionChannel domain.IngestionChannel
	ApplicationID    string
	SourceID         string
	EnvironmentID    string
}

// RegisterTechnicalEventUseCase validates, enriches and persists a technical event.
type RegisterTechnicalEventUseCase struct {
	repo ports.TechnicalEventRepository
	now  func() time.Time
}

func NewRegisterTechnicalEventUseCase(repo ports.TechnicalEventRepository) *RegisterTechnicalEventUseCase {
	return &RegisterTechnicalEventUseCase{
		repo: repo,
		now:  time.Now,
	}
}

func (uc *RegisterTechnicalEventUseCase) Execute(ctx context.Context, input RegisterTechnicalEventInput) (*domain.TechnicalEvent, error) {
	if uc == nil || uc.repo == nil {
		return nil, fmt.Errorf("technical event repository is required")
	}

	if err := validateRegisterTechnicalEventInput(input); err != nil {
		return nil, err
	}

	payload := input.Payload
	if payload == nil {
		payload = map[string]any{}
	}

	category := input.Category
	if category == "" {
		category = domain.CategoryApplication
	}

	severity := deriveSeverity(strings.TrimSpace(input.Message), category, payload)

	event := &domain.TechnicalEvent{
		ExternalID:       strings.TrimSpace(input.ExternalID),
		OccurredAt:       input.OccurredAt,
		ReceivedAt:       uc.now().UTC(),
		Message:          strings.TrimSpace(input.Message),
		Severity:         severity,
		Category:         category,
		Payload:          payload,
		TraceID:          strings.TrimSpace(input.TraceID),
		CorrelationID:    strings.TrimSpace(input.CorrelationID),
		IngestionChannel: input.IngestionChannel,
		ApplicationID:    strings.TrimSpace(input.ApplicationID),
		SourceID:         strings.TrimSpace(input.SourceID),
		EnvironmentID:    strings.TrimSpace(input.EnvironmentID),
	}

	if err := uc.repo.Save(ctx, event); err != nil {
		return nil, fmt.Errorf("failed to register technical event: %w", err)
	}

	return event, nil
}

func validateRegisterTechnicalEventInput(input RegisterTechnicalEventInput) error {
	if input.OccurredAt.IsZero() {
		return fmt.Errorf("occurred_at is required")
	}
	if strings.TrimSpace(input.Message) == "" {
		return fmt.Errorf("message is required")
	}
	if strings.TrimSpace(input.ApplicationID) == "" {
		return fmt.Errorf("application_id is required")
	}
	if strings.TrimSpace(input.SourceID) == "" {
		return fmt.Errorf("source_id is required")
	}
	if strings.TrimSpace(input.EnvironmentID) == "" {
		return fmt.Errorf("environment_id is required")
	}
	if input.IngestionChannel == "" {
		return fmt.Errorf("ingestion_channel is required")
	}
	if !isValidIngestionChannel(input.IngestionChannel) {
		return fmt.Errorf("ingestion_channel is invalid")
	}

	return nil
}

func isValidSeverity(value domain.Severity) bool {
	value = domain.NormalizeSeverity(value)

	switch value {
	case domain.SeverityDebug,
		domain.SeverityInfo,
		domain.SeverityWarn,
		domain.SeverityError,
		domain.SeverityFatal:
		return true
	default:
		return false
	}
}

func isValidIngestionChannel(value domain.IngestionChannel) bool {
	switch value {
	case domain.IngestionHTTP,
		domain.IngestionCLI,
		domain.IngestionFile:
		return true
	default:
		return false
	}
}

func deriveSeverity(message string, category domain.EventCategory, payload map[string]any) domain.Severity {
	content := strings.ToLower(strings.TrimSpace(message))

	if containsAny(content, "panic", "fatal", "unrecoverable", "crash") || payloadHasSignal(payload, "panic", "fatal", "stack_trace") {
		return domain.SeverityFatal
	}

	if containsAny(content, "error", "failed", "failure", "timeout", "exception", "unauthorized", "denied") ||
		payloadHasSignal(payload, "error", "errors", "exception", "exception_message") {
		return domain.SeverityError
	}

	if category == domain.CategorySecurity ||
		containsAny(content, "warn", "warning", "retry", "deprecated", "slow", "fallback") ||
		payloadHasSignal(payload, "warning", "warnings", "retry_count") {
		return domain.SeverityWarn
	}

	return domain.SeverityInfo
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}

	return false
}

func payloadHasSignal(payload map[string]any, keys ...string) bool {
	if len(payload) == 0 {
		return false
	}

	for _, key := range keys {
		value, ok := payload[key]
		if !ok {
			continue
		}

		if hasMeaningfulSignal(value) {
			return true
		}
	}

	return false
}

func hasMeaningfulSignal(value any) bool {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) != ""
	case bool:
		return v
	case int:
		return v > 0
	case int8:
		return v > 0
	case int16:
		return v > 0
	case int32:
		return v > 0
	case int64:
		return v > 0
	case uint:
		return v > 0
	case uint8:
		return v > 0
	case uint16:
		return v > 0
	case uint32:
		return v > 0
	case uint64:
		return v > 0
	case float32:
		return v > 0
	case float64:
		return v > 0
	default:
		rv := reflect.ValueOf(value)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array, reflect.Map:
			return rv.Len() > 0
		default:
			return value != nil
		}
	}
}
