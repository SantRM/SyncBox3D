-- =============================================================================
-- 0003_escenas.up.sql
-- Nivel 2: Laboratorios (escenas multi-equipo) con instancias que guardan
-- un snapshot de la metadata del equipo y su transform en el espacio 3D.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- ESCENA  (un "laboratorio")
-- -----------------------------------------------------------------------------
CREATE TABLE escena (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nombre       varchar(150) NOT NULL UNIQUE,
    descripcion  varchar(500) NOT NULL DEFAULT '',
    activo       boolean      NOT NULL DEFAULT true,
    deleted_at   timestamptz,
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now(),
    created_by   uuid REFERENCES usuario(id),
    updated_by   uuid REFERENCES usuario(id)
);
CREATE INDEX idx_escena_activo ON escena(activo) WHERE deleted_at IS NULL;

-- -----------------------------------------------------------------------------
-- ESCENA_INSTANCIA  (una "máquina colocada" dentro de un laboratorio)
-- equipo_origen_id es nullable + ON DELETE SET NULL: si el equipo original se
-- elimina, la instancia sobrevive con su snapshot textual (sin modelo 3D).
-- El transform actual pertenece a esta instancia del laboratorio; no muta el
-- equipo/modelo principal.
-- pos_inicial_* y escala_inicial guardan el estado al momento de insertar la
-- instancia y nunca se actualizan: el botón "Restore" del UI vuelve a ellos.
-- -----------------------------------------------------------------------------
CREATE TABLE escena_instancia (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    escena_id           uuid NOT NULL REFERENCES escena(id) ON DELETE CASCADE,
    equipo_origen_id    uuid REFERENCES equipo(id) ON DELETE SET NULL,
    orden               int  NOT NULL,

    -- Snapshot textual del equipo al momento del alta.
    nombre_snapshot       varchar(150) NOT NULL,
    fabricante_snapshot   varchar(120) NOT NULL DEFAULT '',
    modelo_snapshot       varchar(120) NOT NULL DEFAULT '',
    categoria_snapshot    varchar(100) NOT NULL DEFAULT '',

    -- Transform actual (mutable).
    pos_x   double precision NOT NULL DEFAULT 0,
    pos_y   double precision NOT NULL DEFAULT 0,
    pos_z   double precision NOT NULL DEFAULT 0,
    escala  double precision NOT NULL DEFAULT 1 CHECK (escala > 0),

    -- Transform inicial (inmutable, usado por "Restore").
    pos_inicial_x   double precision NOT NULL DEFAULT 0,
    pos_inicial_y   double precision NOT NULL DEFAULT 0,
    pos_inicial_z   double precision NOT NULL DEFAULT 0,
    escala_inicial  double precision NOT NULL DEFAULT 1 CHECK (escala_inicial > 0),

    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_escena_instancia_escena ON escena_instancia(escena_id);
CREATE INDEX idx_escena_instancia_equipo ON escena_instancia(equipo_origen_id);
CREATE UNIQUE INDEX uq_escena_instancia_orden ON escena_instancia(escena_id, orden);
