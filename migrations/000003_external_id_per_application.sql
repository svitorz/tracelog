-- external_id passa a ser único por aplicação, e só quando informado (spec 0001 D1/D8, spec 0002).
-- Seguro para dados existentes: a constraint global já garantia que não há repetição.

BEGIN;

ALTER TABLE technical_event
    DROP CONSTRAINT IF EXISTS uq_technical_event_external_id;

CREATE UNIQUE INDEX IF NOT EXISTS uq_technical_event_app_external_id
    ON technical_event (application_id, external_id)
    WHERE external_id IS NOT NULL;

COMMIT;
