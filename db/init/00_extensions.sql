-- Extensiones requeridas por el modelo de datos.
-- Se ejecuta automáticamente la primera vez que el volumen de datos está vacío.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS ltree;
