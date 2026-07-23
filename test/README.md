# Pruebas de integracion Syncbox

Esta carpeta contiene una suite pequena con `node:test` para validar el sistema
levantado por Docker.

## Requisitos

- Node.js 20 o superior.
- Proyecto corriendo, por defecto en `http://localhost:8081`.
- Usuario admin disponible.

## Ejecutar

```powershell
cd test
npm test
```

Variables opcionales:

```powershell
$env:SYNCBOX_FRONTEND_URL = "http://localhost:8081"
$env:SYNCBOX_API_URL = "http://localhost:8081/api/v1"
$env:SYNCBOX_ADMIN_EMAIL = "admin@syncbox.co"
$env:SYNCBOX_ADMIN_PASSWORD = "Cambiar.123!"
npm test
```

## Cobertura actual

1. Frontend: confirma que la SPA, los scripts principales y el logo construido
   se sirven correctamente.
2. Autenticacion: confirma proteccion de `/me`, login admin, payload publico y
   persistencia de idioma del usuario.
3. Arbol de nodos: confirma que una ubicacion puede contener laboratorio, que
   laboratorio no contiene subnodos y que `EQUIPO` no se crea directamente desde
   `/nodos`.
