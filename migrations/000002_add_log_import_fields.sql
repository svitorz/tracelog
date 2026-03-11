BEGIN;

ALTER TABLE technical_event
    ADD COLUMN IF NOT EXISTS raw_line TEXT,
    ADD COLUMN IF NOT EXISTS normalization_status VARCHAR(50) NOT NULL DEFAULT 'normalized',
    ADD COLUMN IF NOT EXISTS parse_error TEXT;

CREATE INDEX IF NOT EXISTS idx_technical_event_normalization_status
    ON technical_event (normalization_status);

COMMIT;
