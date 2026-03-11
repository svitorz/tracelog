package services

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// RegisterTechnicalEventInput defines the input payload for registering a technical event.
type RegisterTechnicalEventInput struct {
	ExternalID       string
	OccurredAt       string
	Message          string
	Category         domain.EventCategory
	Payload          map[string]any
	TraceID          string
	CorrelationID    string
	IngestionChannel domain.IngestionChannel
	ApplicationID    string
	SourceID         string
	EnvironmentID    string
	RawRequestBody   string
	DecodeError      string
}

// RegisterTechnicalEventUseCase validates, enriches and persists a technical event.
type RegisterTechnicalEventUseCase struct {
	repo      ports.TechnicalEventRepository
	setupRepo ports.SystemSetupRepository
	now       func() time.Time
}

func NewRegisterTechnicalEventUseCase(repo ports.TechnicalEventRepository, setupRepo ...ports.SystemSetupRepository) *RegisterTechnicalEventUseCase {
	var defaultsRepo ports.SystemSetupRepository
	if len(setupRepo) > 0 {
		defaultsRepo = setupRepo[0]
	}

	return &RegisterTechnicalEventUseCase{
		repo:      repo,
		setupRepo: defaultsRepo,
		now:       time.Now,
	}
}

func (uc *RegisterTechnicalEventUseCase) Execute(ctx context.Context, input RegisterTechnicalEventInput) (*domain.TechnicalEvent, error) {
	if uc == nil || uc.repo == nil {
		return nil, fmt.Errorf("technical event repository is required")
	}

	if err := validateRegisterTechnicalEventInput(input); err != nil {
		return nil, err
	}

	receivedAt := uc.now().UTC()
	payload := clonePayload(input.Payload)
	notes := make([]string, 0, 8)

	if strings.TrimSpace(input.DecodeError) != "" {
		notes = append(notes, fmt.Sprintf("request body could not be decoded as JSON: %s", strings.TrimSpace(input.DecodeError)))
	}

	occurredAt, occurredNotes := normalizeOccurredAt(input.OccurredAt, receivedAt)
	notes = append(notes, occurredNotes...)

	category, categoryNotes := normalizeCategory(input.Category)
	notes = append(notes, categoryNotes...)

	message, messageNotes := normalizeMessage(input.Message)
	notes = append(notes, messageNotes...)

	defaults, defaultsErr := uc.loadFallbackSetupIfNeeded(ctx, input)
	if defaultsErr != nil {
		return nil, defaultsErr
	}

	applicationID, sourceID, environmentID, idNotes := normalizeReferenceIDs(input, defaults)
	notes = append(notes, idNotes...)

	enrichPayload(payload, input.RawRequestBody, input.OccurredAt, input.DecodeError, notes)
	severity := deriveSeverity(message, category, payload)

	event := &domain.TechnicalEvent{
		ExternalID:       strings.TrimSpace(input.ExternalID),
		OccurredAt:       occurredAt,
		ReceivedAt:       receivedAt,
		Message:          message,
		Severity:         severity,
		Category:         category,
		Payload:          payload,
		TraceID:          strings.TrimSpace(input.TraceID),
		CorrelationID:    strings.TrimSpace(input.CorrelationID),
		IngestionChannel: input.IngestionChannel,
		ApplicationID:    applicationID,
		SourceID:         sourceID,
		EnvironmentID:    environmentID,
	}

	if err := uc.repo.Save(ctx, event); err != nil {
		if shouldRetryWithFallback(err) {
			recovered, recoveryErr := uc.loadFallbackSetup(ctx)
			if recoveryErr == nil {
				retryNotes := applyFallbackSetup(event, recovered)
				if len(retryNotes) > 0 {
					notes = append(notes, retryNotes...)
					enrichPayload(event.Payload, input.RawRequestBody, input.OccurredAt, input.DecodeError, notes)
				}

				if retryErr := uc.repo.Save(ctx, event); retryErr == nil {
					return event, nil
				} else {
					err = retryErr
				}
			}
		}

		return nil, fmt.Errorf("failed to register technical event: %w", err)
	}

	return event, nil
}

func validateRegisterTechnicalEventInput(input RegisterTechnicalEventInput) error {
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

func normalizeOccurredAt(raw string, fallback time.Time) (time.Time, []string) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return fallback, []string{"occurred_at missing; received_at used as fallback"}
	}

	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}

	for _, format := range formats {
		if parsed, err := time.Parse(format, value); err == nil {
			return parsed.UTC(), nil
		}
	}

	return fallback, []string{"occurred_at invalid; received_at used as fallback"}
}

func normalizeCategory(raw domain.EventCategory) (domain.EventCategory, []string) {
	category := domain.EventCategory(strings.ToLower(strings.TrimSpace(string(raw))))

	switch category {
	case domain.CategoryApplication, domain.CategorySystem, domain.CategorySecurity, domain.CategoryAudit, domain.CategoryUnknown:
		return category, nil
	case "":
		return domain.CategoryUnknown, []string{"category missing; defaulted to unknown"}
	default:
		return domain.CategoryUnknown, []string{fmt.Sprintf("category %q invalid; defaulted to unknown", string(raw))}
	}
}

func normalizeMessage(raw string) (string, []string) {
	message := strings.TrimSpace(raw)
	if message == "" {
		return "event received without message", []string{"message missing; fallback message stored"}
	}

	return message, nil
}

func clonePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return map[string]any{}
	}

	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		cloned[key] = value
	}

	return cloned
}

func enrichPayload(payload map[string]any, rawBody string, rawOccurredAt string, decodeError string, notes []string) {
	if payload == nil {
		return
	}

	if len(notes) == 0 && strings.TrimSpace(decodeError) == "" {
		return
	}

	ingestion := map[string]any{}
	if existing, ok := payload["_ingestion"].(map[string]any); ok {
		for key, value := range existing {
			ingestion[key] = value
		}
	}

	if trimmed := strings.TrimSpace(rawBody); trimmed != "" {
		ingestion["raw_request_body"] = trimmed
	}
	if trimmed := strings.TrimSpace(decodeError); trimmed != "" {
		ingestion["decode_error"] = trimmed
	}
	if trimmed := strings.TrimSpace(rawOccurredAt); trimmed != "" {
		ingestion["original_occurred_at"] = trimmed
	}
	if len(notes) > 0 {
		ingestion["normalization_warnings"] = dedupeStrings(notes)
	}

	payload["_ingestion"] = ingestion
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}

func normalizeReferenceIDs(input RegisterTechnicalEventInput, defaults *ports.BasicSystemSetup) (string, string, string, []string) {
	notes := make([]string, 0, 3)

	applicationID := strings.TrimSpace(input.ApplicationID)
	if !isUUIDLike(applicationID) && defaults != nil {
		if applicationID == "" {
			notes = append(notes, "application_id missing; fallback system application used")
		} else {
			notes = append(notes, "application_id invalid; fallback system application used")
		}
		applicationID = defaults.Application.ID
	}

	sourceID := strings.TrimSpace(input.SourceID)
	if !isUUIDLike(sourceID) && defaults != nil {
		if sourceID == "" {
			notes = append(notes, "source_id missing; fallback system source used")
		} else {
			notes = append(notes, "source_id invalid; fallback system source used")
		}
		sourceID = defaults.Source.ID
	}

	environmentID := strings.TrimSpace(input.EnvironmentID)
	if !isUUIDLike(environmentID) && defaults != nil {
		if environmentID == "" {
			notes = append(notes, "environment_id missing; fallback system environment used")
		} else {
			notes = append(notes, "environment_id invalid; fallback system environment used")
		}
		environmentID = defaults.Environment.ID
	}

	return applicationID, sourceID, environmentID, notes
}

func (uc *RegisterTechnicalEventUseCase) loadFallbackSetupIfNeeded(ctx context.Context, input RegisterTechnicalEventInput) (*ports.BasicSystemSetup, error) {
	if isUUIDLike(strings.TrimSpace(input.ApplicationID)) &&
		isUUIDLike(strings.TrimSpace(input.SourceID)) &&
		isUUIDLike(strings.TrimSpace(input.EnvironmentID)) {
		return nil, nil
	}

	return uc.loadFallbackSetup(ctx)
}

func (uc *RegisterTechnicalEventUseCase) loadFallbackSetup(ctx context.Context) (*ports.BasicSystemSetup, error) {
	if uc.setupRepo == nil {
		return nil, fmt.Errorf("fallback system setup is unavailable")
	}

	setup, err := uc.setupRepo.LoadBasicSetup(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load fallback system setup: %w", err)
	}

	return setup, nil
}

func applyFallbackSetup(event *domain.TechnicalEvent, defaults *ports.BasicSystemSetup) []string {
	if event == nil || defaults == nil {
		return nil
	}

	notes := make([]string, 0, 3)
	if event.ApplicationID != defaults.Application.ID {
		event.ApplicationID = defaults.Application.ID
		notes = append(notes, "application_id not persisted; fallback system application used")
	}
	if event.SourceID != defaults.Source.ID {
		event.SourceID = defaults.Source.ID
		notes = append(notes, "source_id not persisted; fallback system source used")
	}
	if event.EnvironmentID != defaults.Environment.ID {
		event.EnvironmentID = defaults.Environment.ID
		notes = append(notes, "environment_id not persisted; fallback system environment used")
	}

	return notes
}

var uuidLikePattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func isUUIDLike(value string) bool {
	return uuidLikePattern.MatchString(strings.TrimSpace(value))
}

func shouldRetryWithFallback(err error) bool {
	if err == nil {
		return false
	}

	message := err.Error()
	return strings.Contains(message, "application_id does not reference an existing application") ||
		strings.Contains(message, "source_id does not reference an existing source") ||
		strings.Contains(message, "environment_id does not reference an existing environment")
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
