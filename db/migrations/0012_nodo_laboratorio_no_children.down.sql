-- Restaura la regla anterior: EQUIPO podia colgar de UBICACION o LABORATORIO.

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
        IF parent_tipo = 'EQUIPO' THEN
            RAISE EXCEPTION 'EQUIPO no puede ser padre de otros nodos';
        END IF;
        IF NEW.tipo = 'UBICACION' AND parent_tipo <> 'UBICACION' THEN
            RAISE EXCEPTION 'UBICACION solo puede tener padre UBICACION';
        END IF;
        IF NEW.tipo = 'LABORATORIO' AND parent_tipo <> 'UBICACION' THEN
            RAISE EXCEPTION 'LABORATORIO solo puede colgar de UBICACION';
        END IF;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
