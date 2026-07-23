-- =============================================================================
-- 0015_enforce_equipment_under_location.up.sql
-- Regla final del arbol:
--   UBICACION puede contener UBICACION, LABORATORIO y EQUIPO.
--   LABORATORIO y EQUIPO son hojas dentro del arbol.
--
-- Los laboratorios consumen equipos por escena_instancia; no son propietarios
-- jerarquicos de esos equipos.
-- =============================================================================

-- Si algun dato historico dejo equipos colgados de un laboratorio, moverlos a
-- la ubicacion padre del laboratorio antes de endurecer la regla.
UPDATE nodo child
SET parent_id = lab.parent_id
FROM nodo lab
WHERE child.parent_id = lab.id
  AND lab.tipo = 'LABORATORIO'
  AND child.tipo = 'EQUIPO'
  AND lab.parent_id IS NOT NULL
  AND child.deleted_at IS NULL;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM nodo child
        JOIN nodo parent ON parent.id = child.parent_id
        WHERE child.deleted_at IS NULL
          AND parent.deleted_at IS NULL
          AND parent.tipo <> 'UBICACION'
    ) THEN
        RAISE EXCEPTION 'No se puede endurecer nodo: solo UBICACION puede contener hijos vivos';
    END IF;
END;
$$;

CREATE OR REPLACE FUNCTION nodo_validate_parent_type()
RETURNS trigger AS $$
DECLARE
    parent_tipo nodo_tipo;
    parent_deleted timestamptz;
BEGIN
    IF NEW.parent_id IS NULL THEN
        IF NEW.tipo <> 'UBICACION' THEN
            RAISE EXCEPTION 'Solo UBICACION puede ser raiz (parent_id = NULL)';
        END IF;
    ELSE
        SELECT tipo, deleted_at
        INTO parent_tipo, parent_deleted
        FROM nodo
        WHERE id = NEW.parent_id;

        IF parent_tipo IS NULL THEN
            RAISE EXCEPTION 'parent_id % no existe', NEW.parent_id;
        END IF;

        IF parent_deleted IS NOT NULL THEN
            RAISE EXCEPTION 'parent_id % esta eliminado', NEW.parent_id;
        END IF;

        IF parent_tipo <> 'UBICACION' THEN
            RAISE EXCEPTION 'Solo UBICACION puede contener nodos hijos';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
