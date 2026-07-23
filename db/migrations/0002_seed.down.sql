-- Reverso de 0002_seed.up.sql
DELETE FROM usuario       WHERE correo = 'admin@syncbox.co';
DELETE FROM alerta_config WHERE estado_id IN (
    SELECT id FROM estado_operativo
    WHERE nombre IN ('Mantenimiento','Fuera de servicio','En uso')
);
DELETE FROM estado_operativo WHERE nombre IN
    ('Disponible','En uso','Mantenimiento','Fuera de servicio');
DELETE FROM categoria WHERE nombre IN (
    'Soldadura','Corte','Elevación / manipulación',
    'Compresión / energía','Instrumentación / medición'
);
