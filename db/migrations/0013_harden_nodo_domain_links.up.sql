-- Endurece la relación entre el árbol `nodo` y las tablas de dominio:
--   - equipo vivo siempre apunta a un nodo EQUIPO vivo.
--   - escena viva siempre apunta a un nodo LABORATORIO vivo.
--   - un nodo EQUIPO/LABORATORIO no se puede reutilizar por dos registros vivos.
--   - el soft-delete se sincroniza entre el nodo y su registro de dominio.

DO $$
DECLARE
    root_id uuid;
    rec record;
    new_node_id uuid;
BEGIN
    SELECT id INTO root_id
    FROM nodo
    WHERE parent_id IS NULL AND slug = 'sin_clasificar' AND deleted_at IS NULL
    LIMIT 1;

    IF root_id IS NULL THEN
        INSERT INTO nodo (tipo, parent_id, nombre, slug, orden, path)
        VALUES ('UBICACION', NULL, 'Sin clasificar', 'sin_clasificar', 0, ''::ltree)
        RETURNING id INTO root_id;
    END IF;

    FOR rec IN
        SELECT id, nombre, deleted_at, activo
        FROM equipo
        WHERE nodo_id IS NULL
    LOOP
        INSERT INTO nodo (tipo, parent_id, nombre, slug, orden, path, activo, deleted_at)
        VALUES (
            'EQUIPO',
            root_id,
            rec.nombre,
            'equipo_migrado_' || replace(rec.id::text, '-', '_'),
            0,
            ''::ltree,
            rec.deleted_at IS NULL AND rec.activo,
            rec.deleted_at
        )
        RETURNING id INTO new_node_id;

        UPDATE equipo
        SET nodo_id = new_node_id
        WHERE id = rec.id;
    END LOOP;

    FOR rec IN
        SELECT id, nombre, deleted_at, activo
        FROM escena
        WHERE nodo_id IS NULL
    LOOP
        INSERT INTO nodo (tipo, parent_id, nombre, slug, orden, path, activo, deleted_at)
        VALUES (
            'LABORATORIO',
            root_id,
            rec.nombre,
            'laboratorio_migrado_' || replace(rec.id::text, '-', '_'),
            0,
            ''::ltree,
            rec.deleted_at IS NULL AND rec.activo,
            rec.deleted_at
        )
        RETURNING id INTO new_node_id;

        UPDATE escena
        SET nodo_id = new_node_id
        WHERE id = rec.id;
    END LOOP;
END;
$$;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM equipo e
        LEFT JOIN nodo n ON n.id = e.nodo_id
        WHERE e.deleted_at IS NULL
          AND (e.nodo_id IS NULL OR n.id IS NULL OR n.deleted_at IS NOT NULL OR n.tipo <> 'EQUIPO')
    ) THEN
        RAISE EXCEPTION 'No se puede endurecer equipo.nodo_id: hay equipos vivos sin nodo EQUIPO vivo';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM escena e
        LEFT JOIN nodo n ON n.id = e.nodo_id
        WHERE e.deleted_at IS NULL
          AND (e.nodo_id IS NULL OR n.id IS NULL OR n.deleted_at IS NOT NULL OR n.tipo <> 'LABORATORIO')
    ) THEN
        RAISE EXCEPTION 'No se puede endurecer escena.nodo_id: hay escenas vivas sin nodo LABORATORIO vivo';
    END IF;

    IF EXISTS (
        SELECT 1
        FROM equipo
        WHERE deleted_at IS NULL
        GROUP BY nodo_id
        HAVING COUNT(*) > 1
    ) THEN
        RAISE EXCEPTION 'No se puede endurecer equipo.nodo_id: hay nodos EQUIPO reutilizados por varios equipos vivos';
    END IF;
END;
$$;

ALTER TABLE equipo
    ALTER COLUMN nodo_id SET NOT NULL;

ALTER TABLE escena
    ALTER COLUMN nodo_id SET NOT NULL;

ALTER TABLE escena
    DROP CONSTRAINT IF EXISTS escena_nodo_id_fkey,
    ADD CONSTRAINT escena_nodo_id_fkey
        FOREIGN KEY (nodo_id) REFERENCES nodo(id) ON DELETE RESTRICT;

CREATE UNIQUE INDEX IF NOT EXISTS uq_equipo_nodo
    ON equipo(nodo_id)
    WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION equipo_validate_nodo_equipo()
RETURNS trigger AS $$
DECLARE
    node_tipo nodo_tipo;
    node_deleted timestamptz;
BEGIN
    IF NEW.deleted_at IS NULL THEN
        IF NEW.nodo_id IS NULL THEN
            RAISE EXCEPTION 'equipo.nodo_id es obligatorio para equipos vivos';
        END IF;

        SELECT tipo, deleted_at
        INTO node_tipo, node_deleted
        FROM nodo
        WHERE id = NEW.nodo_id;

        IF node_tipo IS NULL OR node_deleted IS NOT NULL THEN
            RAISE EXCEPTION 'equipo.nodo_id debe apuntar a un nodo vivo';
        END IF;

        IF node_tipo <> 'EQUIPO' THEN
            RAISE EXCEPTION 'equipo.nodo_id debe apuntar a un nodo EQUIPO';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_equipo_validate_nodo_equipo ON equipo;
CREATE TRIGGER trg_equipo_validate_nodo_equipo
    BEFORE INSERT OR UPDATE OF nodo_id, deleted_at ON equipo
    FOR EACH ROW EXECUTE FUNCTION equipo_validate_nodo_equipo();

CREATE OR REPLACE FUNCTION escena_validate_nodo_laboratorio()
RETURNS trigger AS $$
DECLARE
    node_tipo nodo_tipo;
    node_deleted timestamptz;
BEGIN
    IF NEW.deleted_at IS NULL THEN
        IF NEW.nodo_id IS NULL THEN
            RAISE EXCEPTION 'escena.nodo_id es obligatorio para escenas vivas';
        END IF;

        SELECT tipo, deleted_at
        INTO node_tipo, node_deleted
        FROM nodo
        WHERE id = NEW.nodo_id;

        IF node_tipo IS NULL OR node_deleted IS NOT NULL THEN
            RAISE EXCEPTION 'escena.nodo_id debe apuntar a un nodo vivo';
        END IF;

        IF node_tipo <> 'LABORATORIO' THEN
            RAISE EXCEPTION 'escena.nodo_id debe apuntar a un nodo LABORATORIO';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_escena_validate_nodo_laboratorio ON escena;
CREATE TRIGGER trg_escena_validate_nodo_laboratorio
    BEFORE INSERT OR UPDATE OF nodo_id, deleted_at ON escena
    FOR EACH ROW EXECUTE FUNCTION escena_validate_nodo_laboratorio();

CREATE OR REPLACE FUNCTION sync_soft_delete_from_nodo()
RETURNS trigger AS $$
BEGIN
    IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
        IF NEW.tipo = 'EQUIPO' THEN
            UPDATE equipo
            SET deleted_at = NEW.deleted_at, activo = false
            WHERE nodo_id = NEW.id AND deleted_at IS NULL;
        ELSIF NEW.tipo = 'LABORATORIO' THEN
            UPDATE escena
            SET deleted_at = NEW.deleted_at, activo = false
            WHERE nodo_id = NEW.id AND deleted_at IS NULL;
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_nodo_sync_soft_delete ON nodo;
CREATE TRIGGER trg_nodo_sync_soft_delete
    AFTER UPDATE OF deleted_at ON nodo
    FOR EACH ROW EXECUTE FUNCTION sync_soft_delete_from_nodo();

CREATE OR REPLACE FUNCTION sync_soft_delete_equipo_nodo()
RETURNS trigger AS $$
BEGIN
    IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
        UPDATE nodo
        SET deleted_at = NEW.deleted_at, activo = false
        WHERE id = NEW.nodo_id AND tipo = 'EQUIPO' AND deleted_at IS NULL;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_equipo_sync_soft_delete_nodo ON equipo;
CREATE TRIGGER trg_equipo_sync_soft_delete_nodo
    AFTER UPDATE OF deleted_at ON equipo
    FOR EACH ROW EXECUTE FUNCTION sync_soft_delete_equipo_nodo();

CREATE OR REPLACE FUNCTION sync_soft_delete_escena_nodo()
RETURNS trigger AS $$
BEGIN
    IF OLD.deleted_at IS NULL AND NEW.deleted_at IS NOT NULL THEN
        UPDATE nodo
        SET deleted_at = NEW.deleted_at, activo = false
        WHERE id = NEW.nodo_id AND tipo = 'LABORATORIO' AND deleted_at IS NULL;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_escena_sync_soft_delete_nodo ON escena;
CREATE TRIGGER trg_escena_sync_soft_delete_nodo
    AFTER UPDATE OF deleted_at ON escena
    FOR EACH ROW EXECUTE FUNCTION sync_soft_delete_escena_nodo();
