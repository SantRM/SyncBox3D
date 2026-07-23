-- =============================================================================
-- 0006_escena_iluminacion_visibilidad.down.sql
-- =============================================================================

UPDATE escena
SET luz_intensidad = 3
WHERE luz_intensidad = 12;

UPDATE escena
SET luz_auto_target = true
WHERE luz_auto_target = false;

ALTER TABLE escena
    ALTER COLUMN luz_intensidad SET DEFAULT 3,
    ALTER COLUMN luz_auto_target SET DEFAULT true;
