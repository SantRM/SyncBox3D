-- Reenumera las instancias existentes para que `orden` represente la posicion
-- actual dentro de cada laboratorio, sin huecos dejados por eliminaciones.
WITH ranked AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY escena_id
            ORDER BY orden, created_at, id
        ) AS rn
    FROM escena_instancia
),
bumped AS (
    UPDATE escena_instancia i
    SET orden = -ranked.rn
    FROM ranked
    WHERE i.id = ranked.id
    RETURNING i.id
)
UPDATE escena_instancia
SET orden = -orden
WHERE orden < 0;
