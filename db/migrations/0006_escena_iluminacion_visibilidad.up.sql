-- =============================================================================
-- 0006_escena_iluminacion_visibilidad.up.sql
-- Ajusta defaults para que el foco sea visible y el objetivo sea independiente
-- salvo que el usuario active seguimiento automatico.
-- =============================================================================

ALTER TABLE escena
    ALTER COLUMN luz_intensidad SET DEFAULT 12,
    ALTER COLUMN luz_auto_target SET DEFAULT false;

UPDATE escena
SET luz_intensidad = 12
WHERE luz_intensidad = 3;

UPDATE escena
SET luz_auto_target = false
WHERE luz_auto_target = true;
