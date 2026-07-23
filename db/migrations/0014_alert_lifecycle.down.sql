-- =============================================================================
-- 0014_alert_lifecycle.down.sql
-- =============================================================================

DROP INDEX IF EXISTS uq_alerta_evento_activa_equipo;
DROP INDEX IF EXISTS idx_alerta_evento_popup;
DROP INDEX IF EXISTS idx_alerta_evento_pendientes;

CREATE INDEX idx_alerta_evento_pendientes
    ON alerta_evento(generada_at DESC) WHERE vista_at IS NULL;

DROP TRIGGER IF EXISTS trg_alerta_evento_updated_at ON alerta_evento;
ALTER TABLE alerta_evento
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS pospuesta_por,
    DROP COLUMN IF EXISTS pospuesta_hasta,
    DROP COLUMN IF EXISTS resolucion_motivo,
    DROP COLUMN IF EXISTS resuelta_por,
    DROP COLUMN IF EXISTS resuelta_at;

DROP TRIGGER IF EXISTS trg_alerta_config_updated_at ON alerta_config;
ALTER TABLE alerta_config
    DROP COLUMN IF EXISTS updated_at,
    DROP COLUMN IF EXISTS activa;
