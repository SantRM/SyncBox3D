ALTER TABLE usuario
    ADD COLUMN idioma_preferido varchar(5) NOT NULL DEFAULT 'es',
    ADD CONSTRAINT chk_usuario_idioma_preferido CHECK (idioma_preferido IN ('es','en'));
