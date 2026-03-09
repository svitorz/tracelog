-- Initial MVP schema for TraceLog
-- Entities: environment, monitored_application, event_source, technical_event

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS environment (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS monitored_application (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(150) NOT NULL,
    service_key VARCHAR(150) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS event_source (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(150) NOT NULL,
    source_type VARCHAR(50) NOT NULL,
    description TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (name, source_type)
);

CREATE TABLE IF NOT EXISTS technical_event (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id VARCHAR(255),
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    message TEXT NOT NULL,
    severity VARCHAR(20) NOT NULL,
    category VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,
    trace_id VARCHAR(128),
    correlation_id VARCHAR(128),
    ingestion_channel VARCHAR(50) NOT NULL,
    application_id UUID NOT NULL,
    source_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_technical_event_application
        FOREIGN KEY (application_id) REFERENCES monitored_application(id),
    CONSTRAINT fk_technical_event_source
        FOREIGN KEY (source_id) REFERENCES event_source(id),
    CONSTRAINT fk_technical_event_environment
        FOREIGN KEY (environment_id) REFERENCES environment(id),
    CONSTRAINT uq_technical_event_external_id UNIQUE (external_id)
);

CREATE INDEX IF NOT EXISTS idx_technical_event_occurred_at
    ON technical_event (occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_technical_event_severity
    ON technical_event (severity);

CREATE INDEX IF NOT EXISTS idx_technical_event_application_id
    ON technical_event (application_id);

CREATE INDEX IF NOT EXISTS idx_technical_event_environment_id
    ON technical_event (environment_id);

CREATE INDEX IF NOT EXISTS idx_technical_event_trace_id
    ON technical_event (trace_id);

CREATE INDEX IF NOT EXISTS idx_technical_event_correlation_id
    ON technical_event (correlation_id);

COMMIT;
