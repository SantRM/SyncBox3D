-- Mantiene el estado previo a 0015, equivalente a la regla reforzada por 0012:
-- solo UBICACION puede contener hijos. No se intenta devolver equipos a
-- laboratorios porque la relacion correcta ahora vive en escena_instancia.

CREATE OR REPLACE FUNCTION nodo_validate_parent_type()
RETURNS trigger AS $$
DECLARE
    parent_tipo nodo_tipo;
BEGIN
    IF NEW.parent_id IS NULL THEN
        IF NEW.tipo <> 'UBICACION' THEN
            RAISE EXCEPTION 'Solo UBICACION puede ser raiz (parent_id = NULL)';
        END IF;
    ELSE
        SELECT tipo INTO parent_tipo FROM nodo WHERE id = NEW.parent_id;
        IF parent_tipo IS NULL THEN
            RAISE EXCEPTION 'parent_id % no existe', NEW.parent_id;
        END IF;
        IF parent_tipo <> 'UBICACION' THEN
            RAISE EXCEPTION 'Solo UBICACION puede contener nodos hijos';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
