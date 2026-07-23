# Backend — Plataforma 3D Syncbox

API REST en Go (Fiber v2) que implementa el dominio definido en el Entregable 2 y el plan de desarrollo.

## Stack

- Go 1.22+
- Fiber v2 (HTTP)
- pgx/v5 (PostgreSQL)
- golang-jwt/jwt/v5 (JWT con `jti` revocable)
- bcrypt (hashing de contraseñas)
- gocron/v2 (job de alertas)
- zerolog (logger estructurado)

## Estructura

```
backend/
├── Dockerfile               Multi-stage (builder Go + runtime alpine mínimo)
├── go.mod
├── cmd/api/main.go          Entry point
└── internal/
    ├── config/              Carga de variables de entorno
    ├── domain/              Entidades y errores de dominio
    ├── repository/          Acceso a Postgres (pgx, queries parametrizadas)
    ├── service/             Casos de uso (auth, usuarios, equipos, alertas)
    ├── token/               Emisión y verificación de JWT
    ├── crypto/              Bcrypt
    ├── logger/              zerolog
    ├── jobs/                Generador periódico de alertas
    └── http/
        ├── server.go        Bootstrap del servidor Fiber
        ├── router.go        Registro de rutas
        ├── middleware/      auth, role, ownership, rate-limit, recover, logger
        └── handler/         Handlers REST
```

## Seguridad — implementación efectiva

| Riesgo | Mitigación en este código |
|---|---|
| **IDOR** | Middleware `Ownership` valida cada acceso a `/equipos/{id}/...` consultando la BD; los IDs son UUID (no enumerables); Operador y Consulta solo ven recursos vigentes. |
| **JWT robo / replay** | Access token corto (30 min) con `jti` registrado en `sesion`. Cada request valida que el `jti` no esté revocado. |
| **Refresh rotativo** | `/auth/refresh` revoca el `jti` viejo y emite uno nuevo en una transacción. |
| **Logout efectivo** | `/auth/logout` marca `revocado_at` del `jti` actual. |
| **Cambio de contraseña** | Revoca **todas** las sesiones del usuario. |
| **Brute-force** | Tabla `intento_login`: tras 5 fallos en 15 min, bloqueo temporal. Mensajes neutrales, tiempo de respuesta uniforme. |
| **Privilege escalation** | Cada operación sensible revalida el rol contra la BD; un Admin no puede degradarse a sí mismo si quedaría el sistema sin admins. |
| **Inyección SQL** | Solo queries parametrizadas vía `pgx`. Prohibido construir SQL con `fmt.Sprintf`. |
| **CSRF** | Auth por header `Authorization: Bearer`. CORS restringido por config. |
| **Secrets** | Variables de entorno; sin defaults inseguros en producción. |

## Variables de entorno

| Variable | Descripción | Default dev |
|---|---|---|
| `APP_ENV` | `dev` o `prod` | `dev` |
| `APP_PORT` | Puerto HTTP | `8080` |
| `DB_DSN` | `postgres://user:pass@host:5432/db?sslmode=disable` | — |
| `JWT_SECRET` | Secreto HMAC (mínimo 32 bytes) | — (obligatorio) |
| `JWT_ACCESS_TTL_MIN` | Vida del access token | `30` |
| `JWT_REFRESH_TTL_HOURS` | Vida del refresh token | `168` (7 d) |
| `JWT_ISSUER` | `iss` del JWT | `syncbox` |
| `JWT_AUDIENCE` | `aud` del JWT | `syncbox-app` |
| `LOGIN_MAX_FAILS` | Fallos antes de bloqueo | `5` |
| `LOGIN_BLOCK_MIN` | Minutos de bloqueo | `15` |
| `CORS_ORIGIN` | Origen permitido | `http://localhost:5173` |
| `LOG_LEVEL` | `debug`/`info`/`warn`/`error` | `info` |

## Uso local

El `docker-compose.yml` raíz orquesta BD + migraciones + backend + frontend.
Ver la guía completa en el README del repo.

Para correr **solo este servicio** con `go run` (usando la BD del compose):

```powershell
# 1. Levantar BD + migraciones desde la raíz
cd ..
docker compose up -d db migrate

# 2. Crear backend\.env (godotenv lo lee del cwd) tomando los valores del .env raíz
# Mínimos requeridos: DB_DSN, JWT_SECRET. Ejemplo:
#   DB_DSN=postgres://syncbox:<password>@localhost:5432/syncbox?sslmode=disable
#   JWT_SECRET=<hex de 64 chars>

# 3. Arrancar el backend
cd .\backend
go mod tidy
go run .\cmd\api
```

## Endpoints (resumen)

| Método | Ruta | Rol mínimo |
|---|---|---|
| GET  | `/health/live` | público |
| GET  | `/health/ready` | público |
| POST | `/api/v1/auth/login` | público |
| POST | `/api/v1/auth/refresh` | autenticado |
| POST | `/api/v1/auth/logout` | autenticado |
| POST | `/api/v1/auth/password` | autenticado |
| GET  | `/api/v1/me` | autenticado |
| GET/POST/PATCH/DELETE | `/api/v1/usuarios` | Admin |
| GET/POST/PATCH/DELETE | `/api/v1/categorias` | GET: cualquiera · resto: Admin |
| GET/POST/PATCH/DELETE | `/api/v1/estados` | GET: cualquiera · resto: Admin |
| GET  | `/api/v1/equipos` | autenticado |
| POST | `/api/v1/equipos` | Admin u Operador |
| GET  | `/api/v1/equipos/{id}` | autenticado |
| PATCH | `/api/v1/equipos/{id}` | Admin u Operador |
| DELETE | `/api/v1/equipos/{id}` | Admin u Operador (soft-delete) |
| PATCH | `/api/v1/equipos/{id}/estado` | Admin u Operador |
| GET  | `/api/v1/equipos/{id}/historial` | autenticado |
| GET  | `/api/v1/alertas/pendientes` | Admin |
| POST | `/api/v1/alertas/{id}/visto` | Admin |
