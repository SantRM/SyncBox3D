-- =============================================================================
-- 0004_escena_rotaciones.down.sql
-- =============================================================================

ALTER TABLE escena_instancia
    DROP COLUMN rot_inicial_z,
    DROP COLUMN rot_inicial_y,
    DROP COLUMN rot_inicial_x,
    DROP COLUMN rot_z,
    DROP COLUMN rot_y,
    DROP COLUMN rot_x;
