-- Refuerza la regla organizacional del arbol:
-- UBICACION contiene UBICACION/LABORATORIO/EQUIPO; LABORATORIO no contiene
-- equipos en el arbol, solo los consume desde escena_instancia.

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
