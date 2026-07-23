#!/usr/bin/env bash
# Restaura un dump generado por backup.sh.
# Uso: ./restore.sh archivo.dump
set -euo pipefail

CONTAINER="${CONTAINER:-syncbox-db}"
SRC="${1:?ruta del dump requerida}"

docker exec -i "$CONTAINER" \
    pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --clean --if-exists < "$SRC"

echo "Restore aplicado desde: $SRC"
