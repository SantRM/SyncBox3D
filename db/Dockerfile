FROM postgres:16-alpine

# Configuración por defecto (sobrescribible vía variables de entorno).
ENV POSTGRES_DB=syncbox \
    POSTGRES_USER=syncbox \
    LANG=es_ES.utf8

# Scripts que se ejecutan automáticamente la primera vez que el volumen
# de datos está vacío. Aquí solo habilitamos extensiones; el esquema y
# los datos se aplican posteriormente vía golang-migrate.
COPY init/ /docker-entrypoint-initdb.d/

HEALTHCHECK --interval=10s --timeout=5s --start-period=20s --retries=5 \
    CMD pg_isready -U "$POSTGRES_USER" -d "$POSTGRES_DB" || exit 1

EXPOSE 5432
