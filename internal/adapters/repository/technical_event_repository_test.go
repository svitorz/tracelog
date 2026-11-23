package repository

import (
	"errors"
	"testing"

	"svitorz/tracelog/internal/core/ports"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestFriendlyForeignKeyError(t *testing.T) {
	tests := []struct {
		name           string
		constraintName string
		want           string
	}{
		{
			name:           "application",
			constraintName: "fk_technical_event_application",
			want:           "application_id does not reference an existing application",
		},
		{
			name:           "source",
			constraintName: "fk_technical_event_source",
			want:           "source_id does not reference an existing source",
		},
		{
			name:           "environment",
			constraintName: "fk_technical_event_environment",
			want:           "environment_id does not reference an existing environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &pgconn.PgError{Code: "23503", ConstraintName: tt.constraintName}
			if got := friendlyForeignKeyError(err); got != tt.want {
				t.Fatalf("friendlyForeignKeyError() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMapSaveErrorWrapsForeignKeyViolation(t *testing.T) {
	err := mapSaveError(&pgconn.PgError{Code: "23503", ConstraintName: "fk_technical_event_source"})
	if !errors.Is(err, ports.ErrReferenceNotFound) {
		t.Fatalf("mapSaveError() = %v, want ports.ErrReferenceNotFound", err)
	}

	other := &pgconn.PgError{Code: "57P01"}
	if got := mapSaveError(other); errors.Is(got, ports.ErrReferenceNotFound) {
		t.Fatalf("mapSaveError(%s) = %v, want it unchanged", other.Code, got)
	}
}
