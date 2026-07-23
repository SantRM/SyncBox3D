-- =============================================================================
-- 0004_escena_rotaciones.up.sql
-- Nivel 3: rotacion persistente de instancias dentro de laboratorios.
-- Three.js trabaja rotaciones Euler en radianes; se almacenan los tres ejes.
-- Igual que posicion y escala, esta rotacion pertenece solo a la instancia.
-- =============================================================================

ALTER TABLE escena_instancia
    ADD COLUMN rot_x double precision NOT NULL DEFAULT 0,
    ADD COLUMN rot_y double precision NOT NULL DEFAULT 0,
    ADD COLUMN rot_z double precision NOT NULL DEFAULT 0,
    ADD COLUMN rot_inicial_x double precision NOT NULL DEFAULT 0,
    ADD COLUMN rot_inicial_y double precision NOT NULL DEFAULT 0,
    ADD COLUMN rot_inicial_z double precision NOT NULL DEFAULT 0;
