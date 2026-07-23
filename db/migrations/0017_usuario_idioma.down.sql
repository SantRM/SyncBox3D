ALTER TABLE usuario
    DROP CONSTRAINT IF EXISTS chk_usuario_idioma_preferido,
    DROP COLUMN IF EXISTS idioma_preferido;
