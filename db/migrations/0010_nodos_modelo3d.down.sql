-- =============================================================================
-- 0003_nodos_modelo3d.down.sql
-- =============================================================================

ALTER TABLE equipo
    DROP COLUMN IF EXISTS modelo_3d_id,
    DROP COLUMN IF EXISTS nodo_id;

DROP TRIGGER IF EXISTS trg_modelo_3d_updated_at ON modelo_3d;
DROP TABLE IF EXISTS modelo_3d;

DROP TRIGGER IF EXISTS trg_nodo_updated_at         ON nodo;
DROP TRIGGER IF EXISTS trg_nodo_propagate_path     ON nodo;
DROP TRIGGER IF EXISTS trg_nodo_set_path           ON nodo;
DROP TRIGGER IF EXISTS trg_nodo_validate_parent_type ON nodo;
DROP FUNCTION IF EXISTS nodo_propagate_path();
DROP FUNCTION IF EXISTS nodo_set_path();
DROP FUNCTION IF EXISTS nodo_validate_parent_type();
DROP TABLE IF EXISTS nodo;
DROP TYPE  IF EXISTS nodo_tipo;
-- Extensión ltree se conserva por si otros componentes la usan.
