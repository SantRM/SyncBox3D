#!/usr/bin/env bash
# Backup completo (esquema + datos) en formato custom de pg_dump.
# Uso: ./backup.sh [destino.dump]
set -euo pipefail

CONTAINER="${CONTAINER:-syncbox-db}"
DEST="${1:-backup-$(date +%Y%m%d-%H%M%S).dump}"

docker exec -i "$CONTAINER" \
    pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc > "$DEST"

echo "Backup escrito en: $DEST"
