-- =============================================================================
-- 0007_lab_sesiones.up.sql
-- Historial de sesiones de laboratorio y ultimo transform por instancia.
-- =============================================================================

CREATE TABLE lab_sesion (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    escena_id           uuid NOT NULL REFERENCES escena(id) ON DELETE CASCADE,
    usuario_id          uuid REFERENCES usuario(id) ON DELETE SET NULL,
    iniciada_at         timestamptz NOT NULL DEFAULT now(),
    cerrada_at          timestamptz,
    ultima_actividad_at timestamptz NOT NULL DEFAULT now(),
    cierre_motivo       varchar(80) NOT NULL DEFAULT ''
);

CREATE INDEX idx_lab_sesion_escena_fecha
    ON lab_sesion(escena_id, (COALESCE(cerrada_at, ultima_actividad_at, iniciada_at)) DESC);
CREATE INDEX idx_lab_sesion_usuario ON lab_sesion(usuario_id);

CREATE TABLE lab_sesion_instancia (
    lab_sesion_id uuid NOT NULL REFERENCES lab_sesion(id) ON DELETE CASCADE,
    instancia_id  uuid NOT NULL REFERENCES escena_instancia(id) ON DELETE CASCADE,

    pos_x   double precision NOT NULL,
    pos_y   double precision NOT NULL,
    pos_z   double precision NOT NULL,
    escala  double precision NOT NULL CHECK (escala > 0),
    rot_x   double precision NOT NULL,
    rot_y   double precision NOT NULL,
    rot_z   double precision NOT NULL,

    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (lab_sesion_id, instancia_id)
);

CREATE INDEX idx_lab_sesion_instancia_instancia
    ON lab_sesion_instancia(instancia_id, updated_at DESC);
