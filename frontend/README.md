# Syncbox 3D — Frontend

Aplicación web (Vue 3 + Vite + TypeScript + Pinia + Vue Router + Three.js) que consume la API del backend Go (`/api/v1`).

## Requisitos

- Node.js 20+
- Backend corriendo en `http://localhost:8080` (o ajustar `VITE_API_PROXY` / `VITE_API_BASE_URL`).

## Arranque local

```powershell
cd frontend
copy .env.example .env
npm install
npm run dev
```

Abre http://localhost:5173. En desarrollo Vite proxia `/api` → backend.
`npm run dev` corre bajo `nodemon`; Vite mantiene HMR para cambios en `src/` y el proceso se reinicia si cambian archivos de configuración.

## Build y Docker

```powershell
npm run build           # bundle estático en dist/
docker build -t syncbox-frontend:local .
docker run --rm -p 8080:8080 syncbox-frontend:local
```

NGINX sirve en `:8080` con cabeceras de seguridad y SPA fallback.

## Coherencia con el backend

| Aspecto | Política |
|---|---|
| Auth | `POST /auth/login` → `{access_token, refresh_token}` en JSON. |
| Access token | Solo en memoria (Pinia, `services/api.ts`). Se pierde al recargar. |
| Refresh token | `sessionStorage` (el backend no emite cookie). Se elimina al cerrar pestaña. |
| Refresh | Interceptor llama `POST /auth/refresh` UNA vez ante 401, reintenta. |
| Roles | `ADMINISTRADOR`, `OPERADOR`, `CONSULTA`. UI los oculta; backend los **valida**. |
| CORS | Backend permite el origen configurado en `CORS_ORIGIN` (por defecto `http://localhost:5173`). |
| Logout | `POST /auth/logout` revoca **todas** las sesiones del usuario (corte limpio). |
| Cambio de contraseña | Revoca todas las sesiones; el frontend será expulsado al login en el próximo request. |

## Visor 3D (Three.js)

Componente `src/components/Viewer3D.vue`:

- Carga `.glb`/`.gltf` con `GLTFLoader`.
- Soporta compresión Draco (`DRACOLoader` apuntando a `/draco/`).
- Auto-encuadre (calcula bounding box y posiciona la cámara).
- `OrbitControls` con damping.
- Botón "Centrar" para reset de cámara.
- Modo Sketchfab: pasa `sketchfabId` y se renderiza un iframe oficial.
- Cleanup completo (`dispose`) en unmount.

### Decoders Draco (opcional)

`frontend/public` no participa del build (`publicDir: false`). Si se van a soportar modelos
comprimidos con Draco, sirve los decoders desde backend/NGINX en `/draco/` o ajusta
`setDecoderPath` a una ruta disponible. Sin esto, los modelos sin Draco se cargan igualmente.

### Modelos `.glb` por equipo

`Viewer3D` recibe un `src` generado por `modelStore`. Los modelos nuevos se suben al catalogo
reutilizable con `POST /api/v1/modelos3d`, se deduplican por hash y el equipo queda enlazado con
`PATCH /api/v1/equipos/{id}/modelo3d`. La lectura del archivo se hace desde
`GET /api/v1/modelos3d/{id}/file`.

Los modelos del aplicativo no deben guardarse en `frontend/public`: quedan registrados por la API
y almacenados por el backend en el volumen/ruta de recursos configurado.

En laboratorios, la vista resuelve y el visor carga modelos con concurrencia maxima de 4 para no
saturar red, memoria ni GPU cuando hay varios equipos pesados en escena.

## Estructura

```
src/
  components/    BaseButton, BaseInput, BaseTable, Badge, Modal, RoleGate, Viewer3D
  layouts/       AppLayout (sidebar + idle-logout)
  router/        rutas + guards (auth + roles)
  services/      api.ts (fetch + refresh), types.ts (espejo del backend)
  stores/        auth.ts (Pinia)
  styles/        tokens.css, main.css
  views/         Login, Dashboard, Equipos, EquipoForm, EquipoDetail, Usuarios, Categorias, Alertas, Perfil
```

## Comandos

```
npm run dev       # desarrollo
npm run build     # tipos + bundle
npm run preview   # servir dist/
```
