-- Índice para as leituras filtradas por severidade da API (spec 0005, seção 6): a lista
-- com severity em ordem (occurred_at, id) e a contagem por severidade no período. Medido
-- com loggen -scale 200 (PROGRESS.md, trilha 2, fase 2): lista severity=error de ~11 ms
-- para ~0,2 ms; contagem de ~35 ms (Seq Scan) para ~12 ms; ~8 µs a mais por linha inserida.

BEGIN;

CREATE INDEX IF NOT EXISTS idx_technical_event_severity_occurred_at
    ON technical_event (severity, occurred_at DESC, id DESC);

COMMIT;
