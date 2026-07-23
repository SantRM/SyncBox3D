-- =============================================================================
-- 0016_lab_audit_events.up.sql
-- Auditoria append-only de cambios sobre modelos dentro de un laboratorio.
--
-- lab_sesion_instancia conserva el ultimo transform por sesion/instancia.
-- lab_audit_event guarda cada evento historico sin sobrescribir anteriores.
-- =============================================================================

CREATE TABLE lab_audit_event (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    lab_sesion_id       uuid REFERENCES lab_sesion(id) ON DELETE SET NULL,
    escena_id           uuid NOT NULL REFERENCES escena(id) ON DELETE CASCADE,
    instancia_id        uuid NOT NULL,
    usuario_id          uuid REFERENCES usuario(id) ON DELETE SET NULL,
    event_type          varchar(30) NOT NULL CHECK (event_type IN ('add','transform','restore','restore_session','remove')),
    fecha               timestamptz NOT NULL DEFAULT now(),

    equipo_origen_id      uuid,
    nombre_snapshot       varchar(150) NOT NULL DEFAULT '',
    fabricante_snapshot   varchar(120) NOT NULL DEFAULT '',
    modelo_snapshot       varchar(120) NOT NULL DEFAULT '',
    categoria_snapshot    varchar(100) NOT NULL DEFAULT '',

    pos_x   double precision NOT NULL,
    pos_y   double precision NOT NULL,
    pos_z   double precision NOT NULL,
    escala  double precision NOT NULL CHECK (escala > 0),
    rot_x   double precision NOT NULL,
    rot_y   double precision NOT NULL,
    rot_z   double precision NOT NULL
);

CREATE INDEX idx_lab_audit_event_escena_fecha
    ON lab_audit_event(escena_id, fecha DESC);
CREATE INDEX idx_lab_audit_event_sesion
    ON lab_audit_event(lab_sesion_id, fecha DESC);
CREATE INDEX idx_lab_audit_event_instancia
    ON lab_audit_event(instancia_id, fecha DESC);

INSERT INTO lab_audit_event (
    lab_sesion_id, escena_id, instancia_id, usuario_id, event_type, fecha,
    equipo_origen_id, nombre_snapshot, fabricante_snapshot, modelo_snapshot, categoria_snapshot,
    pos_x, pos_y, pos_z, escala, rot_x, rot_y, rot_z
)
SELECT
    lsi.lab_sesion_id,
    s.escena_id,
    lsi.instancia_id,
    s.usuario_id,
    'transform',
    lsi.updated_at,
    i.equipo_origen_id,
    COALESCE(i.nombre_snapshot, ''),
    COALESCE(i.fabricante_snapshot, ''),
    COALESCE(i.modelo_snapshot, ''),
    COALESCE(i.categoria_snapshot, ''),
    lsi.pos_x,
    lsi.pos_y,
    lsi.pos_z,
    lsi.escala,
    lsi.rot_x,
    lsi.rot_y,
    lsi.rot_z
FROM lab_sesion_instancia lsi
JOIN lab_sesion s ON s.id = lsi.lab_sesion_id
LEFT JOIN escena_instancia i ON i.id = lsi.instancia_id;
