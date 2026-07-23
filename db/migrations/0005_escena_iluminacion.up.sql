-- =============================================================================
-- 0005_escena_iluminacion.up.sql
-- Iluminacion persistente por laboratorio.
-- La luz pertenece a la escena, no al modelo/equipo original.
-- =============================================================================

ALTER TABLE escena
    ADD COLUMN luz_activa boolean NOT NULL DEFAULT false,
    ADD COLUMN luz_intensidad double precision NOT NULL DEFAULT 3 CHECK (luz_intensidad >= 0),
    ADD COLUMN luz_color varchar(20) NOT NULL DEFAULT '#fff4d6',
    ADD COLUMN luz_pos_x double precision NOT NULL DEFAULT 4,
    ADD COLUMN luz_pos_y double precision NOT NULL DEFAULT 6,
    ADD COLUMN luz_pos_z double precision NOT NULL DEFAULT 4,
    ADD COLUMN luz_target_x double precision NOT NULL DEFAULT 0,
    ADD COLUMN luz_target_y double precision NOT NULL DEFAULT 0,
    ADD COLUMN luz_target_z double precision NOT NULL DEFAULT 0,
    ADD COLUMN luz_angulo double precision NOT NULL DEFAULT 0.55 CHECK (luz_angulo > 0 AND luz_angulo <= 1.57079632679),
    ADD COLUMN luz_penumbra double precision NOT NULL DEFAULT 0.35 CHECK (luz_penumbra >= 0 AND luz_penumbra <= 1),
    ADD COLUMN luz_distancia double precision NOT NULL DEFAULT 30 CHECK (luz_distancia >= 0),
    ADD COLUMN luz_auto_target boolean NOT NULL DEFAULT true;
