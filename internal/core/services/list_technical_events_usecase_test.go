package services

import (
	"context"
	"testing"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

type stubListTechnicalEventRepository struct {
	filters ports.TechnicalEventFilters
}

func (r *stubListTechnicalEventRepository) Save(context.Context, *domain.TechnicalEvent) error {
	return nil
}

func (r *stubListTechnicalEventRepository) FindByFilters(_ context.Context, filters ports.TechnicalEventFilters) ([]domain.TechnicalEvent, error) {
	r.filters = filters
	return []domain.TechnicalEvent{}, nil
}

func TestListTechnicalEventsNormalizesSeverity(t *testing.T) {
	repo := &stubListTechnicalEventRepository{}
	uc := NewListTechnicalEventsUseCase(repo)

	_, err := uc.Execute(context.Background(), ListTechnicalEventsInput{
		Severity: domain.Severity("ERROR"),
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if repo.filters.Severity != domain.SeverityError {
		t.Fatalf("filters severity = %q, want %q", repo.filters.Severity, domain.SeverityError)
	}
}
