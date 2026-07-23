# Base de datos — Plataforma 3D Syncbox

Servicio PostgreSQL 16 dockerizado, con migraciones versionadas y seed mínimo.
Se despliega como contenedor independiente del backend (decisión de arquitectura
documentada en `Plan_Desarrollo.md`, sección 2.1).

## Estructura

```
db/
├── Dockerfile              Imagen basada en postgres:16-alpine + extensiones
├── init/                   Scripts ejecutados al inicializar el volumen
│   └── 00_extensions.sql   Habilita pgcrypto y pg_trgm
├── migrations/             Migraciones versionadas (golang-migrate)
│   ├── 0001_init.up.sql
│   ├── 0001_init.down.sql
│   ├── 0002_seed.up.sql
│   └── 0002_seed.down.sql
└── scripts/
    ├── backup.sh           Dump del esquema y datos
    └── restore.sh          Restaura desde un dump
```

## Uso rápido (desarrollo local)

La BD se levanta como parte del compose raíz junto con backend + frontend
(ver README del repo). Para levantarla aislada:

```powershell
# Desde la raíz del proyecto
docker compose up -d db migrate

# Conectarse
docker exec -it syncbox-db psql -U syncbox -d syncbox
```

Las variables se toman del `.env` raíz (`POSTGRES_DB`, `POSTGRES_USER`,
`POSTGRES_PASSWORD`, `POSTGRES_HOST_PORT`).

## Decisiones

- **PostgreSQL 16 alpine**: ligera, oficial, soporta las extensiones requeridas.
- **Extensiones**: `pgcrypto` para `gen_random_uuid()` y `pg_trgm` para búsqueda
  parcial (catálogo). Se habilitan vía `init/00_extensions.sql` (se ejecuta
  automáticamente la primera vez que se inicializa el volumen).
- **Volumen persistente** (`pgdata`) separado de la imagen → la BD sobrevive
  a redeploys del backend.
- **Migraciones aditivas** versionadas con `golang-migrate`. No se permiten
  migraciones destructivas en `main` sin aprobación explícita.
- **Seed mínimo**: 1 administrador, 5 categorías base, 4 estados operativos
  y configuración de alertas por defecto. La contraseña inicial se entrega
  fuera de banda y debe cambiarse en el primer login.
- **Sin puertos expuestos al host en producción**: el backend habla con la BD
  por la red interna de Docker. En el compose raíz se expone `5432` solo
  para desarrollo (configurable vía `POSTGRES_HOST_PORT`).

## Backups

Los scripts `scripts/backup.sh` y `scripts/restore.sh` utilizan `pg_dump` y
`pg_restore`. En producción se programan vía cron en la VM de datos.
