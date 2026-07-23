-- =============================================================================
-- 0001_init.up.sql
-- Esquema inicial de la Plataforma 3D Syncbox.
-- Coherente con el Entregable 2 — Documento de Diseño Técnico, sección 4.2.
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- -----------------------------------------------------------------------------
-- USUARIO
-- -----------------------------------------------------------------------------
CREATE TABLE usuario (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nombre          varchar(120)  NOT NULL,
    correo          varchar(180)  NOT NULL UNIQUE,
    password_hash   varchar(255)  NOT NULL,
    rol             varchar(20)   NOT NULL CHECK (rol IN ('ADMINISTRADOR','OPERADOR','CONSULTA')),
    activo          boolean       NOT NULL DEFAULT true,
    ultima_sesion   timestamptz,
    created_at      timestamptz   NOT NULL DEFAULT now(),
    updated_at      timestamptz   NOT NULL DEFAULT now(),
    updated_by      uuid REFERENCES usuario(id)
);

-- -----------------------------------------------------------------------------
-- CATEGORIA
-- -----------------------------------------------------------------------------
CREATE TABLE categoria (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nombre       varchar(100) NOT NULL UNIQUE,
    descripcion  varchar(255),
    activo       boolean      NOT NULL DEFAULT true,
    created_at   timestamptz  NOT NULL DEFAULT now(),
    updated_at   timestamptz  NOT NULL DEFAULT now()
);

-- -----------------------------------------------------------------------------
-- ESTADO_OPERATIVO
-- -----------------------------------------------------------------------------
CREATE TABLE estado_operativo (
    id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nombre  varchar(60) NOT NULL UNIQUE,
    color   varchar(7)  NOT NULL DEFAULT '#888888',
    orden   int         NOT NULL DEFAULT 0,
    activo  boolean     NOT NULL DEFAULT true
);

-- -----------------------------------------------------------------------------
-- EQUIPO
-- estado_id + estado_desde: vista en tiempo real (consultable desde catálogo).
-- El historial detallado vive en equipo_estado_historial.
-- -----------------------------------------------------------------------------
CREATE TABLE equipo (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nombre        varchar(150) NOT NULL,
    fabricante    varchar(120),
    modelo        varchar(120),
    serial        varchar(120),
    ubicacion     varchar(180),
    categoria_id  uuid NOT NULL REFERENCES categoria(id),
    estado_id     uuid NOT NULL REFERENCES estado_operativo(id),
    estado_desde  timestamptz  NOT NULL DEFAULT now(),
    activo        boolean      NOT NULL DEFAULT true,
    deleted_at    timestamptz,
    created_at    timestamptz  NOT NULL DEFAULT now(),
    updated_at    timestamptz  NOT NULL DEFAULT now(),
    updated_by    uuid REFERENCES usuario(id)
);
CREATE INDEX idx_equipo_categoria   ON equipo(categoria_id);
CREATE INDEX idx_equipo_estado      ON equipo(estado_id);
CREATE UNIQUE INDEX uq_equipo_serial_activo
    ON equipo(serial) WHERE serial IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX idx_equipo_nombre_trgm ON equipo USING gin (nombre gin_trgm_ops);
CREATE INDEX idx_equipo_serial_trgm ON equipo USING gin (serial gin_trgm_ops);

-- -----------------------------------------------------------------------------
-- FICHA_TECNICA
-- atributos_extra (JSONB) cubre los atributos extensibles por categoría.
-- -----------------------------------------------------------------------------
CREATE TABLE ficha_tecnica (
    equipo_id        uuid PRIMARY KEY REFERENCES equipo(id) ON DELETE CASCADE,
    peso             numeric(10,2),
    potencia         numeric(10,2),
    dimensiones      varchar(80),
    anio             int,
    observaciones    text,
    atributos_extra  jsonb NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX idx_ficha_atributos_gin ON ficha_tecnica USING gin (atributos_extra);

-- -----------------------------------------------------------------------------
-- RECURSO_EQUIPO
-- origen y formato son ortogonales. es_principal: único por equipo+formato.
-- -----------------------------------------------------------------------------
CREATE TABLE recurso_equipo (
    id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    equipo_id      uuid NOT NULL REFERENCES equipo(id) ON DELETE CASCADE,
    origen         varchar(20) NOT NULL CHECK (origen IN ('LOCAL','SKETCHFAB')),
    formato        varchar(20) NOT NULL CHECK (formato IN ('GLB','GLTF','IMAGEN','PDF')),
    url            varchar(500) NOT NULL,
    tamanio_bytes  bigint,
    es_principal   boolean NOT NULL DEFAULT false,
    created_at     timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX uq_recurso_principal_por_formato
    ON recurso_equipo(equipo_id, formato) WHERE es_principal = true;
CREATE INDEX idx_recurso_equipo ON recurso_equipo(equipo_id);

-- -----------------------------------------------------------------------------
-- CAMBIO_HISTORIAL (auditoría general)
-- -----------------------------------------------------------------------------
CREATE TABLE cambio_historial (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    entidad         varchar(40) NOT NULL CHECK (entidad IN
                        ('EQUIPO','USUARIO','CATEGORIA','ESTADO_OPERATIVO',
                         'ALERTA_CONFIG','FICHA_TECNICA','RECURSO_EQUIPO')),
    entidad_id      uuid        NOT NULL,
    usuario_id      uuid        NOT NULL REFERENCES usuario(id),
    campo           varchar(80) NOT NULL,
    valor_anterior  text,
    valor_nuevo     text,
    fecha           timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_historial_entidad_fecha
    ON cambio_historial(entidad, entidad_id, fecha DESC);

-- -----------------------------------------------------------------------------
-- EQUIPO_ESTADO_HISTORIAL
-- -----------------------------------------------------------------------------
CREATE TABLE equipo_estado_historial (
    id                  uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    equipo_id           uuid NOT NULL REFERENCES equipo(id) ON DELETE CASCADE,
    estado_anterior_id  uuid REFERENCES estado_operativo(id),
    estado_nuevo_id     uuid NOT NULL REFERENCES estado_operativo(id),
    usuario_id          uuid NOT NULL REFERENCES usuario(id),
    motivo              varchar(255),
    fecha               timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_estado_hist_equipo_fecha
    ON equipo_estado_historial(equipo_id, fecha DESC);
CREATE INDEX idx_estado_hist_estado_nuevo
    ON equipo_estado_historial(estado_nuevo_id);

-- -----------------------------------------------------------------------------
-- ALERTA_CONFIG / ALERTA_EVENTO
-- -----------------------------------------------------------------------------
CREATE TABLE alerta_config (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    estado_id     uuid NOT NULL REFERENCES estado_operativo(id),
    dias_umbral   int  NOT NULL CHECK (dias_umbral > 0),
    UNIQUE (estado_id)
);

CREATE TABLE alerta_evento (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    equipo_id    uuid NOT NULL REFERENCES equipo(id) ON DELETE CASCADE,
    estado_id    uuid NOT NULL REFERENCES estado_operativo(id),
    generada_at  timestamptz NOT NULL DEFAULT now(),
    vista_at     timestamptz,
    vista_por    uuid REFERENCES usuario(id),
    UNIQUE (equipo_id, estado_id, generada_at)
);
CREATE INDEX idx_alerta_evento_pendientes
    ON alerta_evento(generada_at DESC) WHERE vista_at IS NULL;

-- -----------------------------------------------------------------------------
-- SESION e INTENTO_LOGIN (seguridad)
-- -----------------------------------------------------------------------------
CREATE TABLE sesion (
    jti          uuid PRIMARY KEY,
    usuario_id   uuid NOT NULL REFERENCES usuario(id) ON DELETE CASCADE,
    emitido_at   timestamptz NOT NULL DEFAULT now(),
    expira_at    timestamptz NOT NULL,
    revocado_at  timestamptz
);
CREATE INDEX idx_sesion_usuario ON sesion(usuario_id);
CREATE INDEX idx_sesion_activa  ON sesion(usuario_id) WHERE revocado_at IS NULL;

CREATE TABLE intento_login (
    id      uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    correo  varchar(180) NOT NULL,
    ip      varchar(45),
    exito   boolean NOT NULL,
    fecha   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_intento_login_correo_fecha
    ON intento_login(correo, fecha DESC);

-- -----------------------------------------------------------------------------
-- Trigger genérico para mantener updated_at
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_usuario_updated_at
    BEFORE UPDATE ON usuario
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_categoria_updated_at
    BEFORE UPDATE ON categoria
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TRIGGER trg_equipo_updated_at
    BEFORE UPDATE ON equipo
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
