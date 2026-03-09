-- Migração inicial do MVP TraceLog
-- Entidades: ambiente, aplicacao_monitorada, fonte_evento, evento_tecnico

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS ambiente (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	code VARCHAR(50) NOT NULL UNIQUE,
	name VARCHAR(100) NOT NULL,
	description TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS aplicacao_monitorada (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	name VARCHAR(150) NOT NULL,
	service_key VARCHAR(150) NOT NULL UNIQUE,
	description TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS fonte_evento (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	name VARCHAR(150) NOT NULL,
	source_type VARCHAR(50) NOT NULL,
	description TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	UNIQUE (name, source_type)
);

CREATE TABLE IF NOT EXISTS evento_tecnico (
	id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
	external_id VARCHAR(255) NOT NULL,
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
	CONSTRAINT fk_evento_tecnico_aplicacao
		FOREIGN KEY (application_id) REFERENCES aplicacao_monitorada(id),
	CONSTRAINT fk_evento_tecnico_fonte
		FOREIGN KEY (source_id) REFERENCES fonte_evento(id),
	CONSTRAINT fk_evento_tecnico_ambiente
		FOREIGN KEY (environment_id) REFERENCES ambiente(id),
	CONSTRAINT uq_evento_tecnico_external_id UNIQUE (external_id)
);

CREATE INDEX IF NOT EXISTS idx_evento_tecnico_occurred_at
	ON evento_tecnico (occurred_at DESC);

CREATE INDEX IF NOT EXISTS idx_evento_tecnico_severity
	ON evento_tecnico (severity);

CREATE INDEX IF NOT EXISTS idx_evento_tecnico_application_id
	ON evento_tecnico (application_id);

CREATE INDEX IF NOT EXISTS idx_evento_tecnico_environment_id
	ON evento_tecnico (environment_id);

CREATE INDEX IF NOT EXISTS idx_evento_tecnico_trace_id
	ON evento_tecnico (trace_id);

CREATE INDEX IF NOT EXISTS idx_evento_tecnico_correlation_id
	ON evento_tecnico (correlation_id);

INSERT INTO ambiente (code, name, description)
VALUES
	('local', 'Local', 'Ambiente local de desenvolvimento'),
	('dev', 'Development', 'Ambiente de desenvolvimento compartilhado'),
	('staging', 'Staging', 'Ambiente de homologação'),
	('prod', 'Production', 'Ambiente de produção')
ON CONFLICT (code) DO NOTHING;

COMMIT;
