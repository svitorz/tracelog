package services

import (
	"context"
	"fmt"
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
	Severity         domain.Severity
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

	event := &domain.TechnicalEvent{
		ExternalID:       strings.TrimSpace(input.ExternalID),
		OccurredAt:       input.OccurredAt,
		ReceivedAt:       uc.now().UTC(),
		Message:          strings.TrimSpace(input.Message),
		Severity:         input.Severity,
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
	if input.Severity == "" {
		return fmt.Errorf("severity is required")
	}
	if !isValidSeverity(input.Severity) {
		return fmt.Errorf("severity is invalid")
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
