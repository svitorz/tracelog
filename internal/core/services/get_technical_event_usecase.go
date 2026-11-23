package services

import (
	"context"
	"fmt"
	"strings"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

// GetTechnicalEventUseCase loads one event by id.
type GetTechnicalEventUseCase struct {
	repo ports.TechnicalEventRepository
}

func NewGetTechnicalEventUseCase(repo ports.TechnicalEventRepository) *GetTechnicalEventUseCase {
	return &GetTechnicalEventUseCase{repo: repo}
}

// Execute returns a *ValidationError for a malformed id and an error wrapping
// ports.ErrNotFound when the event does not exist.
func (uc *GetTechnicalEventUseCase) Execute(ctx context.Context, id string) (domain.TechnicalEvent, error) {
	id = strings.TrimSpace(id)
	if !IsUUID(id) {
		return domain.TechnicalEvent{}, invalidField("id", "id must be a UUID")
	}

	event, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return domain.TechnicalEvent{}, fmt.Errorf("failed to get technical event: %w", err)
	}

	return event, nil
}
