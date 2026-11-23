package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"

	"github.com/jackc/pgx/v5/pgconn"
)

const defaultFindLimit = 100

const technicalEventColumns = `
	id,
	external_id,
	occurred_at,
	received_at,
	message,
	severity,
	category,
	payload,
	trace_id,
	correlation_id,
	raw_line,
	normalization_status,
	parse_error,
	ingestion_channel,
	application_id,
	source_id,
	environment_id,
	created_at`

// PostgresTechnicalEventRepository persists technical events in PostgreSQL.
type PostgresTechnicalEventRepository struct {
	db *sql.DB
}

func NewPostgresTechnicalEventRepository(db *sql.DB) *PostgresTechnicalEventRepository {
	return &PostgresTechnicalEventRepository{db: db}
}

// Save inserts the event. When an event with the same (application_id, external_id)
// already exists, nothing is inserted, event is overwritten with the stored row and
// created is false. ON CONFLICT avoids the race of check-then-insert (spec 0001 D9).
func (r *PostgresTechnicalEventRepository) Save(ctx context.Context, event *domain.TechnicalEvent) (bool, error) {
	if event == nil {
		return false, fmt.Errorf("technical event is nil")
	}

	if r.db == nil {
		return false, fmt.Errorf("database connection is nil")
	}

	payload := event.Payload
	if payload == nil {
		payload = map[string]any{}
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("failed to marshal technical event payload: %w", err)
	}

	query := `
		INSERT INTO technical_event (
			id,
			external_id,
			occurred_at,
			received_at,
			message,
			severity,
			category,
			payload,
			trace_id,
			correlation_id,
			raw_line,
			normalization_status,
			parse_error,
			ingestion_channel,
			application_id,
			source_id,
			environment_id
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()),
			NULLIF($2, ''),
			$3,
			COALESCE($4, NOW()),
			$5,
			$6,
			$7,
			$8::jsonb,
			NULLIF($9, ''),
			NULLIF($10, ''),
			NULLIF($11, ''),
			$12,
			NULLIF($13, ''),
			$14,
			$15::uuid,
			$16::uuid,
			$17::uuid
		)
		ON CONFLICT (application_id, external_id) WHERE external_id IS NOT NULL DO NOTHING
		RETURNING id, received_at, created_at
	`

	var receivedAt sql.NullTime
	if !event.ReceivedAt.IsZero() {
		receivedAt = sql.NullTime{Time: event.ReceivedAt, Valid: true}
	}

	err = r.db.QueryRowContext(
		ctx,
		query,
		event.ID,
		event.ExternalID,
		event.OccurredAt,
		receivedAt,
		event.Message,
		string(event.Severity),
		string(event.Category),
		string(payloadJSON),
		event.TraceID,
		event.CorrelationID,
		event.RawLine,
		string(event.NormalizationStatus),
		event.ParseError,
		string(event.IngestionChannel),
		event.ApplicationID,
		event.SourceID,
		event.EnvironmentID,
	).Scan(&event.ID, &event.ReceivedAt, &event.CreatedAt)

	switch {
	case err == nil:
		event.Payload = payload
		return true, nil
	case errors.Is(err, sql.ErrNoRows):
		existing, findErr := r.findByExternalID(ctx, event.ApplicationID, event.ExternalID)
		if findErr != nil {
			return false, fmt.Errorf("failed to load existing technical event: %w", findErr)
		}
		*event = existing
		return false, nil
	default:
		return false, fmt.Errorf("failed to save technical event: %w", mapSaveError(err))
	}
}

func (r *PostgresTechnicalEventRepository) findByExternalID(ctx context.Context, applicationID, externalID string) (domain.TechnicalEvent, error) {
	query := `SELECT ` + technicalEventColumns + `
		FROM technical_event
		WHERE application_id = $1::uuid AND external_id = $2`

	return scanTechnicalEvent(r.db.QueryRowContext(ctx, query, applicationID, externalID))
}

func (r *PostgresTechnicalEventRepository) FindByFilters(ctx context.Context, filters ports.TechnicalEventFilters) ([]domain.TechnicalEvent, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `SELECT ` + technicalEventColumns + `
		FROM technical_event
		WHERE 1=1
	`

	args := make([]any, 0, 6)
	argPos := 1

	if filters.ApplicationID != "" {
		query += fmt.Sprintf(" AND application_id = $%d::uuid", argPos)
		args = append(args, filters.ApplicationID)
		argPos++
	}

	if filters.EnvironmentID != "" {
		query += fmt.Sprintf(" AND environment_id = $%d::uuid", argPos)
		args = append(args, filters.EnvironmentID)
		argPos++
	}

	if filters.Severity != "" {
		query += fmt.Sprintf(" AND severity = $%d", argPos)
		args = append(args, string(filters.Severity))
		argPos++
	}

	if filters.OccurredFrom != nil {
		query += fmt.Sprintf(" AND occurred_at >= $%d", argPos)
		args = append(args, *filters.OccurredFrom)
		argPos++
	}

	if filters.OccurredTo != nil {
		query += fmt.Sprintf(" AND occurred_at <= $%d", argPos)
		args = append(args, *filters.OccurredTo)
		argPos++
	}

	if filters.After != nil {
		// Row comparison matches the ORDER BY below, so equal timestamps are neither
		// skipped nor repeated across pages.
		query += fmt.Sprintf(" AND (occurred_at, id) < ($%d, $%d::uuid)", argPos, argPos+1)
		args = append(args, filters.After.OccurredAt, filters.After.ID)
		argPos += 2
	}

	limit := filters.Limit
	if limit <= 0 {
		limit = defaultFindLimit
	}

	query += fmt.Sprintf(" ORDER BY occurred_at DESC, id DESC LIMIT $%d", argPos)
	args = append(args, limit)
	query = strings.TrimSpace(query)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query technical events: %w", err)
	}
	defer rows.Close()

	events := make([]domain.TechnicalEvent, 0)
	for rows.Next() {
		event, err := scanTechnicalEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating technical event rows: %w", err)
	}

	return events, nil
}

func (r *PostgresTechnicalEventRepository) FindByID(ctx context.Context, id string) (domain.TechnicalEvent, error) {
	if r.db == nil {
		return domain.TechnicalEvent{}, fmt.Errorf("database connection is nil")
	}

	query := `SELECT ` + technicalEventColumns + `
		FROM technical_event
		WHERE id = $1::uuid`

	event, err := scanTechnicalEvent(r.db.QueryRowContext(ctx, query, id))
	if errors.Is(err, sql.ErrNoRows) {
		return domain.TechnicalEvent{}, fmt.Errorf("technical event %s: %w", id, ports.ErrNotFound)
	}
	if err != nil {
		return domain.TechnicalEvent{}, err
	}

	return event, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

// scanTechnicalEvent reads one row selected with technicalEventColumns.
func scanTechnicalEvent(row rowScanner) (domain.TechnicalEvent, error) {
	var (
		event         domain.TechnicalEvent
		externalID    sql.NullString
		severity      string
		category      string
		ingestion     string
		payloadRaw    []byte
		traceID       sql.NullString
		correlationID sql.NullString
		rawLine       sql.NullString
		normalization string
		parseError    sql.NullString
	)

	if err := row.Scan(
		&event.ID,
		&externalID,
		&event.OccurredAt,
		&event.ReceivedAt,
		&event.Message,
		&severity,
		&category,
		&payloadRaw,
		&traceID,
		&correlationID,
		&rawLine,
		&normalization,
		&parseError,
		&ingestion,
		&event.ApplicationID,
		&event.SourceID,
		&event.EnvironmentID,
		&event.CreatedAt,
	); err != nil {
		return domain.TechnicalEvent{}, fmt.Errorf("failed to scan technical event: %w", err)
	}

	event.ExternalID = externalID.String
	event.Severity = domain.Severity(severity)
	event.Category = domain.EventCategory(category)
	event.IngestionChannel = domain.IngestionChannel(ingestion)
	event.TraceID = traceID.String
	event.CorrelationID = correlationID.String
	event.RawLine = rawLine.String
	event.NormalizationStatus = domain.NormalizationStatus(normalization)
	event.ParseError = parseError.String

	event.Payload = map[string]any{}
	if len(payloadRaw) > 0 {
		if err := json.Unmarshal(payloadRaw, &event.Payload); err != nil {
			return domain.TechnicalEvent{}, fmt.Errorf("failed to unmarshal technical event payload: %w", err)
		}
	}

	return event, nil
}

var _ ports.TechnicalEventRepository = (*PostgresTechnicalEventRepository)(nil)

// mapSaveError translates PostgreSQL errors the core layer must react to into port errors.
func mapSaveError(err error) error {
	if message := friendlyForeignKeyError(err); message != "" {
		return fmt.Errorf("%w: %s", ports.ErrReferenceNotFound, message)
	}

	return err
}

func friendlyForeignKeyError(err error) string {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return ""
	}

	switch pgErr.ConstraintName {
	case "fk_technical_event_application":
		return "application_id does not reference an existing application"
	case "fk_technical_event_source":
		return "source_id does not reference an existing source"
	case "fk_technical_event_environment":
		return "environment_id does not reference an existing environment"
	default:
		return "one or more referenced IDs do not exist"
	}
}
