# Despliegue en Proxmox

Guia minima para levantar Syncbox en una VM/LXC con Docker Compose.

## 1. Preparar `.env`

```bash
cp .env.example .env
```

Ajusta como minimo:

```env
APP_ENV=prod
POSTGRES_PASSWORD=<password-fuerte>
JWT_SECRET=<salida-de-openssl-rand-hex-32>
CORS_ORIGIN=http://IP_O_DOMINIO:8081
RESOURCE_HOST_PATH=/srv/syncbox/recursos
MODEL_MAX_MB=500
```

Por seguridad, deja estos valores salvo que necesites exponerlos explicitamente:

```env
POSTGRES_BIND_ADDRESS=127.0.0.1
BACKEND_BIND_ADDRESS=127.0.0.1
FRONTEND_BIND_ADDRESS=0.0.0.0
```

Asi solo queda publicado el frontend. Postgres y backend siguen disponibles en
el host local y dentro de la red Docker, pero no hacia toda la red.

## 2. Crear carpeta persistente para modelos

```bash
sudo mkdir -p /srv/syncbox/recursos
sudo chown -R 100:101 /srv/syncbox/recursos
```

El backend ve esa carpeta como `/data/syncbox/recursos` y guarda ahi los
modelos 3D convertidos/subidos.

## 3. Levantar

```bash
docker compose -f docker-compose.yml -f docker-compose.proxmox.yml up -d --build
```

Verifica:

```bash
docker compose ps
curl http://127.0.0.1:8080/health/ready
```

La aplicacion queda en:

```text
http://IP_O_DOMINIO:8081
```

## 4. Backups

Base de datos:

```bash
docker exec syncbox-db pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" > syncbox-db.sql
```

Modelos 3D:

```bash
sudo tar czf syncbox-recursos.tgz -C /srv/syncbox recursos
```

Para un entorno productivo, programa ambos backups fuera de la VM o en un
almacenamiento respaldado por Proxmox.
