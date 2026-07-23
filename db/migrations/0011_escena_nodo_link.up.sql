-- Vincula los laboratorios 3D (escena) con el arbol de ubicaciones.
-- La escena/laboratorio no contiene equipos en el arbol; consume equipos por
-- escena_instancia. El nodo LABORATORIO solo representa su ubicacion logica.

ALTER TABLE escena
    ADD COLUMN IF NOT EXISTS nodo_id uuid REFERENCES nodo(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_escena_nodo
    ON escena(nodo_id)
    WHERE nodo_id IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_escena_nodo
    ON escena(nodo_id)
    WHERE deleted_at IS NULL;

CREATE OR REPLACE FUNCTION escena_validate_nodo_laboratorio()
RETURNS trigger AS $$
DECLARE
    nodo_tipo_actual nodo_tipo;
BEGIN
    IF NEW.nodo_id IS NULL THEN
        RETURN NEW;
    END IF;

    SELECT tipo INTO nodo_tipo_actual
    FROM nodo
    WHERE id = NEW.nodo_id
      AND deleted_at IS NULL;

    IF nodo_tipo_actual IS NULL THEN
        RAISE EXCEPTION 'nodo_id % no existe o esta eliminado', NEW.nodo_id;
    END IF;
    IF nodo_tipo_actual <> 'LABORATORIO' THEN
        RAISE EXCEPTION 'escena.nodo_id debe apuntar a un nodo LABORATORIO';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trg_escena_validate_nodo_laboratorio ON escena;
CREATE TRIGGER trg_escena_validate_nodo_laboratorio
    BEFORE INSERT OR UPDATE OF nodo_id ON escena
    FOR EACH ROW EXECUTE FUNCTION escena_validate_nodo_laboratorio();

-- Backfill conservador: si ya existe un nodo LABORATORIO con el mismo nombre
-- que la escena y no esta enlazado a otra escena, se enlaza automaticamente.
WITH candidates AS (
    SELECT e.id AS escena_id, n.id AS nodo_id,
           row_number() OVER (PARTITION BY e.id ORDER BY n.path::text) AS rn
    FROM escena e
    JOIN nodo n
      ON n.tipo = 'LABORATORIO'
     AND n.deleted_at IS NULL
     AND lower(n.nombre) = lower(e.nombre)
    WHERE e.deleted_at IS NULL
      AND e.nodo_id IS NULL
      AND NOT EXISTS (
          SELECT 1
          FROM escena e2
          WHERE e2.nodo_id = n.id
            AND e2.deleted_at IS NULL
      )
)
UPDATE escena e
SET nodo_id = c.nodo_id
FROM candidates c
WHERE e.id = c.escena_id
  AND c.rn = 1;
