package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"svitorz/tracelog/internal/core/domain"
	"svitorz/tracelog/internal/core/ports"
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
			ingestion_channel,
			application_id,
			source_id,
			environment_id
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()),
			$2,
			$3,
			COALESCE($4, NOW()),
			$5,
			$6,
			$7,
			$8::jsonb,
			NULLIF($9, ''),
			NULLIF($10, ''),
			$11,
			$12::uuid,
			$13::uuid,
			$14::uuid
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
		string(event.IngestionChannel),
		event.ApplicationID,
		event.SourceID,
		event.EnvironmentID,
	).Scan(&event.ID, &event.ReceivedAt, &event.CreatedAt); err != nil {
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
			severity      string
			category      string
			ingestion     string
			payloadRaw    []byte
			traceID       sql.NullString
			correlationID sql.NullString
		)

		if err := rows.Scan(
			&event.ID,
			&event.ExternalID,
			&event.OccurredAt,
			&event.ReceivedAt,
			&event.Message,
			&severity,
			&category,
			&payloadRaw,
			&traceID,
			&correlationID,
			&ingestion,
			&event.ApplicationID,
			&event.SourceID,
			&event.EnvironmentID,
			&event.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan technical event: %w", err)
		}

		event.Severity = domain.Severity(severity)
		event.Category = domain.EventCategory(category)
		event.IngestionChannel = domain.IngestionChannel(ingestion)
		event.TraceID = traceID.String
		event.CorrelationID = correlationID.String

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
