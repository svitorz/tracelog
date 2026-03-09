package services

import (
	"context"
	"testing"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
)

type stubSystemSetupRepository struct {
	saved ports.BasicSystemSetup
	ok    bool
}

func (r *stubSystemSetupRepository) SaveBasicSetup(_ context.Context, setup ports.BasicSystemSetup) (*ports.BasicSystemSetup, error) {
	r.saved = setup
	r.saved.Application.ID = "app-1"
	r.saved.Environment.ID = "env-1"
	r.saved.Source.ID = "source-1"
	return &r.saved, nil
}

func (r *stubSystemSetupRepository) HasBasicSetup(context.Context) (bool, error) {
	return r.ok, nil
}

func TestInitializeSystemUseCaseSavesNormalizedSetup(t *testing.T) {
	repo := &stubSystemSetupRepository{}
	uc := NewInitializeSystemUseCase(repo)

	setup, err := uc.Execute(context.Background(), InitializeSystemInput{
		ApplicationName:        " Local App ",
		ApplicationServiceKey:  " local-app ",
		ApplicationDescription: " default app ",
		EnvironmentCode:        " local ",
		EnvironmentName:        " Local ",
		EnvironmentDescription: " local env ",
		SourceName:             " HTTP API ",
		SourceType:             " http ",
		SourceDescription:      " ingress ",
	})
	if err != nil {
		t.Fatalf("Execute() unexpected error = %v", err)
	}

	if setup.Application.Name != "Local App" {
		t.Fatalf("Application.Name = %q, want %q", setup.Application.Name, "Local App")
	}
	if repo.saved.Environment.Code != "local" {
		t.Fatalf("Environment.Code = %q, want %q", repo.saved.Environment.Code, "local")
	}
	if repo.saved.Source.SourceType != "http" {
		t.Fatalf("Source.SourceType = %q, want %q", repo.saved.Source.SourceType, "http")
	}
}

func TestInitializeSystemUseCaseRequiresMinimumFields(t *testing.T) {
	repo := &stubSystemSetupRepository{}
	uc := NewInitializeSystemUseCase(repo)

	_, err := uc.Execute(context.Background(), InitializeSystemInput{
		ApplicationServiceKey: "local-app",
		EnvironmentCode:       "local",
		EnvironmentName:       "Local",
		SourceName:            "HTTP API",
		SourceType:            "http",
	})
	if err == nil {
		t.Fatal("Execute() error = nil, want validation error")
	}
}

func TestEnsureSystemSetupUseCaseReturnsErrorWhenSetupIsMissing(t *testing.T) {
	repo := &stubSystemSetupRepository{ok: false}
	uc := NewEnsureSystemSetupUseCase(repo)

	err := uc.Execute(context.Background())
	if err == nil {
		t.Fatal("Execute() error = nil, want missing setup error")
	}
}

var _ ports.SystemSetupRepository = (*stubSystemSetupRepository)(nil)
var _ = domain.Environment{}
