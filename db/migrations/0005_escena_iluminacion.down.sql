-- =============================================================================
-- 0005_escena_iluminacion.down.sql
-- =============================================================================

ALTER TABLE escena
    DROP COLUMN luz_auto_target,
    DROP COLUMN luz_distancia,
    DROP COLUMN luz_penumbra,
    DROP COLUMN luz_angulo,
    DROP COLUMN luz_target_z,
    DROP COLUMN luz_target_y,
    DROP COLUMN luz_target_x,
    DROP COLUMN luz_pos_z,
    DROP COLUMN luz_pos_y,
    DROP COLUMN luz_pos_x,
    DROP COLUMN luz_color,
    DROP COLUMN luz_intensidad,
    DROP COLUMN luz_activa;
