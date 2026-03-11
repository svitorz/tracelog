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

// PostgresTechnicalEventRepository persists technical events in PostgreSQL.
type PostgresTechnicalEventRepository struct {
	db *sql.DB
}

func NewPostgresTechnicalEventRepository(db *sql.DB) *PostgresTechnicalEventRepository {
	return &PostgresTechnicalEventRepository{db: db}
}

func (r *PostgresTechnicalEventRepository) Save(ctx context.Context, event *domain.TechnicalEvent) error {
	if event == nil {
		return fmt.Errorf("technical event is nil")
	}

	if r.db == nil {
		return fmt.Errorf("database connection is nil")
	}

	payload := event.Payload
	if payload == nil {
		payload = map[string]any{}
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal technical event payload: %w", err)
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
		RETURNING id, received_at, created_at
	`

	var receivedAt sql.NullTime
	if !event.ReceivedAt.IsZero() {
		receivedAt = sql.NullTime{Time: event.ReceivedAt, Valid: true}
	}

	if err := r.db.QueryRowContext(
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
	).Scan(&event.ID, &event.ReceivedAt, &event.CreatedAt); err != nil {
		if message := friendlyForeignKeyError(err); message != "" {
			return fmt.Errorf("failed to save technical event: %s", message)
		}

		return fmt.Errorf("failed to save technical event: %w", err)
	}

	event.Payload = payload
	return nil
}

func (r *PostgresTechnicalEventRepository) FindByFilters(ctx context.Context, filters ports.TechnicalEventFilters) ([]domain.TechnicalEvent, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}

	query := `
		SELECT
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
			created_at
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

	limit := filters.Limit
	if limit <= 0 {
		limit = defaultFindLimit
	}

	query += fmt.Sprintf(" ORDER BY occurred_at DESC LIMIT $%d", argPos)
	args = append(args, limit)
	query = strings.TrimSpace(query)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query technical events: %w", err)
	}
	defer rows.Close()

	events := make([]domain.TechnicalEvent, 0)
	for rows.Next() {
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

		if err := rows.Scan(
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
			return nil, fmt.Errorf("failed to scan technical event: %w", err)
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

		if len(payloadRaw) == 0 {
			event.Payload = map[string]any{}
		} else {
			if err := json.Unmarshal(payloadRaw, &event.Payload); err != nil {
				return nil, fmt.Errorf("failed to unmarshal technical event payload: %w", err)
			}
		}

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed iterating technical event rows: %w", err)
	}

	return events, nil
}

var _ ports.TechnicalEventRepository = (*PostgresTechnicalEventRepository)(nil)

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
