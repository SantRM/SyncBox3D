-- Reverso de 0001_init.up.sql
DROP TRIGGER IF EXISTS trg_equipo_updated_at    ON equipo;
DROP TRIGGER IF EXISTS trg_categoria_updated_at ON categoria;
DROP TRIGGER IF EXISTS trg_usuario_updated_at   ON usuario;
DROP FUNCTION IF EXISTS set_updated_at();

DROP TABLE IF EXISTS intento_login;
DROP TABLE IF EXISTS sesion;
DROP TABLE IF EXISTS alerta_evento;
DROP TABLE IF EXISTS alerta_config;
DROP TABLE IF EXISTS equipo_estado_historial;
DROP TABLE IF EXISTS cambio_historial;
DROP TABLE IF EXISTS recurso_equipo;
DROP TABLE IF EXISTS ficha_tecnica;
DROP TABLE IF EXISTS equipo;
DROP TABLE IF EXISTS estado_operativo;
DROP TABLE IF EXISTS categoria;
DROP TABLE IF EXISTS usuario;
