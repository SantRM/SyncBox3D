-- =============================================================================
-- 0014_alert_lifecycle.up.sql
-- Ciclo de vida real para alertas: configurables, pospuestas y resueltas.
-- =============================================================================

ALTER TABLE alerta_config
    ADD COLUMN IF NOT EXISTS activa boolean NOT NULL DEFAULT true,
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

CREATE TRIGGER trg_alerta_config_updated_at
    BEFORE UPDATE ON alerta_config
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

ALTER TABLE alerta_evento
    ADD COLUMN IF NOT EXISTS resuelta_at timestamptz,
    ADD COLUMN IF NOT EXISTS resuelta_por uuid REFERENCES usuario(id),
    ADD COLUMN IF NOT EXISTS resolucion_motivo varchar(120) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS pospuesta_hasta timestamptz,
    ADD COLUMN IF NOT EXISTS pospuesta_por uuid REFERENCES usuario(id),
    ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

CREATE TRIGGER trg_alerta_evento_updated_at
    BEFORE UPDATE ON alerta_evento
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

UPDATE alerta_evento
SET resuelta_at = vista_at,
    resuelta_por = vista_por,
    resolucion_motivo = 'vista_migrada'
WHERE vista_at IS NOT NULL
  AND resuelta_at IS NULL;

WITH ranked AS (
    SELECT id,
           ROW_NUMBER() OVER (
               PARTITION BY equipo_id
               ORDER BY generada_at DESC, id DESC
           ) AS rn
    FROM alerta_evento
    WHERE resuelta_at IS NULL
)
UPDATE alerta_evento ae
SET resuelta_at = now(),
    resolucion_motivo = 'deduplicada_migracion'
FROM ranked r
WHERE ae.id = r.id
  AND r.rn > 1;

DROP INDEX IF EXISTS idx_alerta_evento_pendientes;
CREATE INDEX idx_alerta_evento_pendientes
    ON alerta_evento(generada_at DESC)
    WHERE resuelta_at IS NULL;

CREATE INDEX idx_alerta_evento_popup
    ON alerta_evento((COALESCE(pospuesta_hasta, generada_at)), generada_at DESC)
    WHERE resuelta_at IS NULL;

CREATE UNIQUE INDEX uq_alerta_evento_activa_equipo
    ON alerta_evento(equipo_id)
    WHERE resuelta_at IS NULL;

INSERT INTO alerta_config (estado_id, dias_umbral, activa)
SELECT eo.id,
       CASE
           WHEN lower(eo.nombre) = 'mantenimiento' THEN 15
           WHEN lower(eo.nombre) = 'fuera de servicio' THEN 30
           ELSE 1
       END,
       CASE
           WHEN lower(eo.nombre) IN ('disponible', 'en uso') THEN false
           ELSE true
       END
FROM estado_operativo eo
ON CONFLICT (estado_id) DO UPDATE
SET activa = CASE
        WHEN lower((SELECT nombre FROM estado_operativo WHERE id = EXCLUDED.estado_id)) IN ('disponible', 'en uso') THEN false
        ELSE alerta_config.activa
    END,
    dias_umbral = CASE
        WHEN lower((SELECT nombre FROM estado_operativo WHERE id = EXCLUDED.estado_id)) = 'en uso' THEN alerta_config.dias_umbral
        ELSE alerta_config.dias_umbral
    END;
