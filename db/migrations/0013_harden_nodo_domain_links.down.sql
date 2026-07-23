DROP TRIGGER IF EXISTS trg_escena_sync_soft_delete_nodo ON escena;
DROP FUNCTION IF EXISTS sync_soft_delete_escena_nodo();

DROP TRIGGER IF EXISTS trg_equipo_sync_soft_delete_nodo ON equipo;
DROP FUNCTION IF EXISTS sync_soft_delete_equipo_nodo();

DROP TRIGGER IF EXISTS trg_nodo_sync_soft_delete ON nodo;
DROP FUNCTION IF EXISTS sync_soft_delete_from_nodo();

DROP TRIGGER IF EXISTS trg_equipo_validate_nodo_equipo ON equipo;
DROP FUNCTION IF EXISTS equipo_validate_nodo_equipo();

DROP TRIGGER IF EXISTS trg_escena_validate_nodo_laboratorio ON escena;

DROP INDEX IF EXISTS uq_equipo_nodo;

ALTER TABLE escena
    DROP CONSTRAINT IF EXISTS escena_nodo_id_fkey,
    ALTER COLUMN nodo_id DROP NOT NULL,
    ADD CONSTRAINT escena_nodo_id_fkey
        FOREIGN KEY (nodo_id) REFERENCES nodo(id) ON DELETE SET NULL;

ALTER TABLE equipo
    ALTER COLUMN nodo_id DROP NOT NULL;

CREATE OR REPLACE FUNCTION escena_validate_nodo_laboratorio()
RETURNS trigger AS $$
DECLARE
    node_tipo nodo_tipo;
BEGIN
    IF NEW.nodo_id IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT tipo INTO node_tipo
    FROM nodo
    WHERE id = NEW.nodo_id
      AND deleted_at IS NULL;

    IF node_tipo IS NULL THEN
        RAISE EXCEPTION 'nodo_id % no existe o esta eliminado', NEW.nodo_id;
    END IF;

    IF node_tipo <> 'LABORATORIO' THEN
        RAISE EXCEPTION 'escena.nodo_id debe apuntar a un nodo LABORATORIO';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_escena_validate_nodo_laboratorio
    BEFORE INSERT OR UPDATE OF nodo_id ON escena
    FOR EACH ROW EXECUTE FUNCTION escena_validate_nodo_laboratorio();
