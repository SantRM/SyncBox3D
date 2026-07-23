-- =============================================================================
-- 0002_seed.up.sql
-- Datos iniciales: 1 admin, 5 categorías base, 4 estados operativos,
-- y configuración de alertas por defecto.
--
-- IMPORTANTE: la contraseña del admin está hasheada con bcrypt (cost 12)
-- y corresponde al texto plano "Cambiar.123!". DEBE cambiarse en el primer
-- login. La política de claves se aplica desde el backend.
-- =============================================================================

-- -----------------------------------------------------------------------------
-- Categorías base
-- -----------------------------------------------------------------------------
INSERT INTO categoria (nombre, descripcion) VALUES
    ('Soldadura',                 'Equipos de soldadura y fusión.'),
    ('Corte',                     'Equipos de corte mecánico, plasma o láser.'),
    ('Elevación / manipulación',  'Polipastos, grúas, montacargas y similares.'),
    ('Compresión / energía',      'Compresores, generadores y suministro de energía.'),
    ('Instrumentación / medición','Instrumentos de medición y control.')
ON CONFLICT (nombre) DO NOTHING;

-- -----------------------------------------------------------------------------
-- Estados operativos
-- -----------------------------------------------------------------------------
INSERT INTO estado_operativo (nombre, color, orden) VALUES
    ('Disponible',         '#2E7D32', 1),
    ('En uso',             '#1F3A5F', 2),
    ('Mantenimiento',      '#C77700', 3),
    ('Fuera de servicio',  '#B3261E', 4)
ON CONFLICT (nombre) DO NOTHING;

-- -----------------------------------------------------------------------------
-- Configuración de alertas por defecto (umbral en días)
-- -----------------------------------------------------------------------------
INSERT INTO alerta_config (estado_id, dias_umbral)
SELECT id, 15 FROM estado_operativo WHERE nombre = 'Mantenimiento'
ON CONFLICT (estado_id) DO NOTHING;

INSERT INTO alerta_config (estado_id, dias_umbral)
SELECT id, 30 FROM estado_operativo WHERE nombre = 'Fuera de servicio'
ON CONFLICT (estado_id) DO NOTHING;

INSERT INTO alerta_config (estado_id, dias_umbral)
SELECT id, 60 FROM estado_operativo WHERE nombre = 'En uso'
ON CONFLICT (estado_id) DO NOTHING;

-- -----------------------------------------------------------------------------
-- Administrador inicial
-- correo:    admin@syncbox.co
-- password:  Cambiar.123!   (bcrypt, debe rotarse en el primer login)
-- -----------------------------------------------------------------------------
INSERT INTO usuario (nombre, correo, password_hash, rol)
VALUES (
    'Administrador Inicial',
    'admin@syncbox.co',
    '$2a$12$4DaQe2u3gSY3QyarUDAuKudSBOHWuZFUNrgXDpX1vMMeu1.2Kc0K2',
    'ADMINISTRADOR'
)
ON CONFLICT (correo) DO NOTHING;
