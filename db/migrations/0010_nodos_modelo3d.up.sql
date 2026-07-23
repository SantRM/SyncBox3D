-- =============================================================================
-- 0003_nodos_modelo3d.up.sql
-- Reestructura jerárquica: árbol UBICACION/LABORATORIO/EQUIPO con `ltree`,
-- catálogo reusable `modelo_3d` y enlace desde `equipo`.
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS ltree;

-- -----------------------------------------------------------------------------
-- ENUM nodo_tipo
-- -----------------------------------------------------------------------------
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'nodo_tipo') THEN
        CREATE TYPE nodo_tipo AS ENUM ('UBICACION', 'LABORATORIO', 'EQUIPO');
    END IF;
END$$;

-- -----------------------------------------------------------------------------
-- NODO
-- Árbol jerárquico. `path` se mantiene por trigger desde (parent_id, slug).
-- Reglas de tipo:
--   - UBICACION: parent NULL o UBICACION.
--   - LABORATORIO: parent UBICACION (obligatorio).
--   - EQUIPO: parent UBICACION o LABORATORIO (obligatorio); siempre hoja.
-- -----------------------------------------------------------------------------
CREATE TABLE nodo (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tipo        nodo_tipo NOT NULL,
    parent_id   uuid REFERENCES nodo(id) ON DELETE RESTRICT,
    nombre      varchar(180) NOT NULL,
    slug        varchar(180) NOT NULL,
    orden       int NOT NULL DEFAULT 0,
    path        ltree NOT NULL,
    depth       int GENERATED ALWAYS AS (nlevel(path)) STORED,
    activo      boolean NOT NULL DEFAULT true,
    deleted_at  timestamptz,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    created_by  uuid REFERENCES usuario(id),
    updated_by  uuid REFERENCES usuario(id),
    CONSTRAINT nodo_slug_chk CHECK (slug ~ '^[a-z0-9_]+$')
);

CREATE INDEX idx_nodo_path_gist  ON nodo USING gist (path);
CREATE INDEX idx_nodo_parent     ON nodo(parent_id);
CREATE INDEX idx_nodo_tipo_act   ON nodo(tipo) WHERE deleted_at IS NULL;
CREATE UNIQUE INDEX uq_nodo_parent_slug
    ON nodo(COALESCE(parent_id, '00000000-0000-0000-0000-000000000000'::uuid), slug)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_nodo_nombre_trgm ON nodo USING gin (nombre gin_trgm_ops);

-- -----------------------------------------------------------------------------
-- Trigger: validar tipo según parent
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION nodo_validate_parent_type()
RETURNS trigger AS $$
DECLARE
    parent_tipo nodo_tipo;
BEGIN
    IF NEW.parent_id IS NULL THEN
        IF NEW.tipo <> 'UBICACION' THEN
            RAISE EXCEPTION 'Solo UBICACION puede ser raíz (parent_id = NULL)';
        END IF;
    ELSE
        SELECT tipo INTO parent_tipo FROM nodo WHERE id = NEW.parent_id;
        IF parent_tipo IS NULL THEN
            RAISE EXCEPTION 'parent_id % no existe', NEW.parent_id;
        END IF;
        IF parent_tipo = 'EQUIPO' THEN
            RAISE EXCEPTION 'EQUIPO no puede ser padre de otros nodos';
        END IF;
        IF NEW.tipo = 'UBICACION' AND parent_tipo <> 'UBICACION' THEN
            RAISE EXCEPTION 'UBICACION solo puede tener padre UBICACION';
        END IF;
        IF NEW.tipo = 'LABORATORIO' AND parent_tipo <> 'UBICACION' THEN
            RAISE EXCEPTION 'LABORATORIO solo puede colgar de UBICACION';
        END IF;
        -- EQUIPO puede colgar de UBICACION o LABORATORIO: ok.
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_nodo_validate_parent_type
    BEFORE INSERT OR UPDATE OF parent_id, tipo ON nodo
    FOR EACH ROW EXECUTE FUNCTION nodo_validate_parent_type();

-- -----------------------------------------------------------------------------
-- Trigger: mantener path desde (parent_id, slug) y propagar a descendientes
-- -----------------------------------------------------------------------------
CREATE OR REPLACE FUNCTION nodo_set_path()
RETURNS trigger AS $$
DECLARE
    parent_path ltree;
BEGIN
    IF NEW.parent_id IS NULL THEN
        NEW.path := NEW.slug::ltree;
    ELSE
        SELECT path INTO parent_path FROM nodo WHERE id = NEW.parent_id;
        IF parent_path IS NULL THEN
            RAISE EXCEPTION 'parent_id % no existe', NEW.parent_id;
        END IF;
        NEW.path := parent_path || NEW.slug::ltree;
    END IF;

    -- Anti-ciclo: nunca el nuevo path puede ser ancestro del antiguo si es UPDATE.
    IF TG_OP = 'UPDATE' AND OLD.path IS NOT NULL AND NEW.path <> OLD.path THEN
        IF NEW.path <@ OLD.path AND NEW.path <> OLD.path THEN
            RAISE EXCEPTION 'Movimiento crearía un ciclo en el árbol';
        END IF;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_nodo_set_path
    BEFORE INSERT OR UPDATE OF parent_id, slug ON nodo
    FOR EACH ROW EXECUTE FUNCTION nodo_set_path();

CREATE OR REPLACE FUNCTION nodo_propagate_path()
RETURNS trigger AS $$
BEGIN
    IF OLD.path IS DISTINCT FROM NEW.path THEN
        UPDATE nodo
        SET path = NEW.path || subpath(path, nlevel(OLD.path))
        WHERE path <@ OLD.path AND id <> NEW.id;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_nodo_propagate_path
    AFTER UPDATE OF path ON nodo
    FOR EACH ROW EXECUTE FUNCTION nodo_propagate_path();

CREATE TRIGGER trg_nodo_updated_at
    BEFORE UPDATE ON nodo
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- -----------------------------------------------------------------------------
-- MODELO_3D
-- Catálogo reusable de modelos. sha256 garantiza dedup.
-- -----------------------------------------------------------------------------
CREATE TABLE modelo_3d (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    nombre        varchar(180) NOT NULL,
    descripcion   text,
    mime          varchar(80)  NOT NULL DEFAULT 'model/gltf-binary',
    tamano_bytes  bigint       NOT NULL,
    sha256        char(64)     NOT NULL UNIQUE,
    storage_uri   text         NOT NULL,
    preview_uri   text,
    activo        boolean      NOT NULL DEFAULT true,
    created_at    timestamptz  NOT NULL DEFAULT now(),
    updated_at    timestamptz  NOT NULL DEFAULT now(),
    created_by    uuid REFERENCES usuario(id),
    updated_by    uuid REFERENCES usuario(id)
);

CREATE INDEX idx_modelo3d_nombre_trgm ON modelo_3d USING gin (nombre gin_trgm_ops);

CREATE TRIGGER trg_modelo_3d_updated_at
    BEFORE UPDATE ON modelo_3d
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- -----------------------------------------------------------------------------
-- Enlace desde EQUIPO. Mantenemos el campo legacy `ubicacion` (text) por ahora.
-- -----------------------------------------------------------------------------
ALTER TABLE equipo
    ADD COLUMN nodo_id      uuid REFERENCES nodo(id) ON DELETE RESTRICT,
    ADD COLUMN modelo_3d_id uuid REFERENCES modelo_3d(id) ON DELETE SET NULL;

CREATE INDEX idx_equipo_nodo     ON equipo(nodo_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_equipo_modelo3d ON equipo(modelo_3d_id) WHERE deleted_at IS NULL;

-- -----------------------------------------------------------------------------
-- Backfill: nodo raíz "Sin clasificar" + una UBICACION por cada `equipo.ubicacion`
-- distinto, y vincular `equipo.nodo_id`. Slug se normaliza con regex simple.
-- -----------------------------------------------------------------------------
DO $$
DECLARE
    root_id uuid;
    rec     record;
    sl      text;
    nid     uuid;
BEGIN
    -- Raíz "Sin clasificar".
    INSERT INTO nodo (tipo, parent_id, nombre, slug)
    VALUES ('UBICACION', NULL, 'Sin clasificar', 'sin_clasificar')
    ON CONFLICT DO NOTHING
    RETURNING id INTO root_id;

    IF root_id IS NULL THEN
        SELECT id INTO root_id FROM nodo
        WHERE parent_id IS NULL AND slug = 'sin_clasificar';
    END IF;

    -- Una UBICACION por cada valor distinto no vacío.
    FOR rec IN
        SELECT DISTINCT TRIM(ubicacion) AS ub
        FROM equipo
        WHERE ubicacion IS NOT NULL AND TRIM(ubicacion) <> ''
    LOOP
        sl := lower(regexp_replace(rec.ub, '[^a-zA-Z0-9]+', '_', 'g'));
        sl := trim(both '_' from sl);
        IF sl = '' THEN
            sl := 'ubicacion_' || substr(md5(rec.ub), 1, 8);
        END IF;

        INSERT INTO nodo (tipo, parent_id, nombre, slug)
        VALUES ('UBICACION', root_id, rec.ub, sl)
        ON CONFLICT DO NOTHING;
    END LOOP;

    -- Asignar a cada equipo un nodo EQUIPO bajo su UBICACION (o raíz).
    FOR rec IN SELECT id, nombre, ubicacion FROM equipo WHERE deleted_at IS NULL AND nodo_id IS NULL
    LOOP
        IF rec.ubicacion IS NOT NULL AND TRIM(rec.ubicacion) <> '' THEN
            sl := lower(regexp_replace(TRIM(rec.ubicacion), '[^a-zA-Z0-9]+', '_', 'g'));
            sl := trim(both '_' from sl);
            SELECT id INTO nid FROM nodo
            WHERE parent_id = root_id AND slug = sl AND deleted_at IS NULL
            LIMIT 1;
        ELSE
            nid := root_id;
        END IF;

        IF nid IS NULL THEN
            nid := root_id;
        END IF;

        -- Crear nodo EQUIPO bajo esa UBICACION.
        INSERT INTO nodo (tipo, parent_id, nombre, slug)
        VALUES ('EQUIPO', nid,
                rec.nombre,
                'eq_' || replace(rec.id::text, '-', ''))
        RETURNING id INTO nid;

        UPDATE equipo SET nodo_id = nid WHERE id = rec.id;
    END LOOP;
END$$;
