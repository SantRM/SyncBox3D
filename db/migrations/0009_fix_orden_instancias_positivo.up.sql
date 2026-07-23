-- Corrige instalaciones que hayan quedado con `orden` temporal negativo y
-- vuelve a compactar por escena con sentencias separadas.
UPDATE escena_instancia
SET orden = ABS(orden)
WHERE orden < 0;

WITH ranked AS (
    SELECT
        id,
        ROW_NUMBER() OVER (
            PARTITION BY escena_id
            ORDER BY orden, created_at, id
        ) AS rn
    FROM escena_instancia
)
UPDATE escena_instancia i
SET orden = -ranked.rn
FROM ranked
WHERE i.id = ranked.id;

UPDATE escena_instancia
SET orden = -orden
WHERE orden < 0;
