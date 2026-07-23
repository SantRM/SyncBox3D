BEGIN;

DO $$
DECLARE
    suffix text := substr(md5(clock_timestamp()::text), 1, 12);
    cat_id uuid;
    state_id uuid;
    location_node_id uuid;
    equipment_node_id uuid;
    lab_node_id uuid;
    equipment_id uuid;
    scene_id uuid;
    rejected boolean;
BEGIN
    INSERT INTO categoria (nombre, descripcion)
    VALUES ('integrity category ' || suffix, 'temporary integrity check')
    RETURNING id INTO cat_id;

    INSERT INTO estado_operativo (nombre, color, orden)
    VALUES ('integrity state ' || suffix, '#123456', 999)
    RETURNING id INTO state_id;

    INSERT INTO nodo (tipo, parent_id, nombre, slug, orden, path)
    VALUES ('UBICACION', NULL, 'Integrity Root ' || suffix, 'integrity_root_' || suffix, 0, ''::ltree)
    RETURNING id INTO location_node_id;

    INSERT INTO nodo (tipo, parent_id, nombre, slug, orden, path)
    VALUES ('EQUIPO', location_node_id, 'Integrity Equipment Node ' || suffix, 'integrity_equipment_' || suffix, 0, ''::ltree)
    RETURNING id INTO equipment_node_id;

    INSERT INTO nodo (tipo, parent_id, nombre, slug, orden, path)
    VALUES ('LABORATORIO', location_node_id, 'Integrity Lab Node ' || suffix, 'integrity_lab_' || suffix, 0, ''::ltree)
    RETURNING id INTO lab_node_id;

    rejected := false;
    BEGIN
        INSERT INTO equipo (nombre, categoria_id, estado_id)
        VALUES ('Equipment without node ' || suffix, cat_id, state_id);
    EXCEPTION WHEN OTHERS THEN
        rejected := true;
    END;
    IF NOT rejected THEN
        RAISE EXCEPTION 'expected live equipo without nodo_id to be rejected';
    END IF;

    rejected := false;
    BEGIN
        INSERT INTO equipo (nombre, categoria_id, estado_id, nodo_id)
        VALUES ('Equipment on location node ' || suffix, cat_id, state_id, location_node_id);
    EXCEPTION WHEN OTHERS THEN
        rejected := true;
    END;
    IF NOT rejected THEN
        RAISE EXCEPTION 'expected live equipo on non-EQUIPO node to be rejected';
    END IF;

    INSERT INTO equipo (nombre, categoria_id, estado_id, nodo_id)
    VALUES ('Valid Equipment ' || suffix, cat_id, state_id, equipment_node_id)
    RETURNING id INTO equipment_id;

    rejected := false;
    BEGIN
        INSERT INTO equipo (nombre, categoria_id, estado_id, nodo_id)
        VALUES ('Duplicate Equipment Node ' || suffix, cat_id, state_id, equipment_node_id);
    EXCEPTION WHEN OTHERS THEN
        rejected := true;
    END;
    IF NOT rejected THEN
        RAISE EXCEPTION 'expected duplicate live equipo.nodo_id to be rejected';
    END IF;

    rejected := false;
    BEGIN
        INSERT INTO escena (nombre, descripcion, nodo_id)
        VALUES ('Scene without node ' || suffix, 'temporary integrity check', NULL);
    EXCEPTION WHEN OTHERS THEN
        rejected := true;
    END;
    IF NOT rejected THEN
        RAISE EXCEPTION 'expected live escena without nodo_id to be rejected';
    END IF;

    INSERT INTO escena (nombre, descripcion, nodo_id)
    VALUES ('Valid Scene ' || suffix, 'temporary integrity check', lab_node_id)
    RETURNING id INTO scene_id;

    UPDATE nodo SET deleted_at = now(), activo = false WHERE id = equipment_node_id;
    PERFORM 1 FROM equipo WHERE id = equipment_id AND deleted_at IS NOT NULL AND activo = false;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'expected soft-deleting EQUIPO node to soft-delete linked equipo';
    END IF;

    UPDATE nodo SET deleted_at = now(), activo = false WHERE id = lab_node_id;
    PERFORM 1 FROM escena WHERE id = scene_id AND deleted_at IS NOT NULL AND activo = false;
    IF NOT FOUND THEN
        RAISE EXCEPTION 'expected soft-deleting LABORATORIO node to soft-delete linked escena';
    END IF;
END $$;

ROLLBACK;
