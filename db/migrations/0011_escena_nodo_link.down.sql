DROP INDEX IF EXISTS idx_escena_nodo;
DROP INDEX IF EXISTS uq_escena_nodo;

DROP TRIGGER IF EXISTS trg_escena_validate_nodo_laboratorio ON escena;
DROP FUNCTION IF EXISTS escena_validate_nodo_laboratorio();

ALTER TABLE escena
    DROP COLUMN IF EXISTS nodo_id;
