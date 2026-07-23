# Uso Basico de SyncBox 3D

SyncBox 3D permite gestionar ubicaciones, equipos, laboratorios, modelos 3D y
alertas operativas desde una aplicacion web.

## Acceso

Con el proyecto levantado, abre:

```text
http://localhost:8081
```

En servidor/Proxmox, cambia `localhost` por la IP o dominio configurado.

## Usuario semilla

El sistema crea un usuario administrador inicial:

```text
Correo: admin@syncbox.co
Contrasena: Cambiar.123!
Rol: ADMINISTRADOR
```

Este usuario es solo para el primer acceso. Debe cambiarse la contrasena al
entrar por primera vez.

## Cambiar el usuario o la contrasena

Para cambiar la contrasena del usuario semilla:

1. Inicia sesion con `admin@syncbox.co`.
2. Entra a `Mi cuenta`.
3. Usa la seccion de cambio de contrasena.
4. Cierra sesion e ingresa nuevamente con la nueva clave.

Para reemplazar el usuario semilla por otro administrador:

1. Entra con el usuario semilla.
2. Ve a `Usuarios`.
3. Crea un nuevo usuario con rol `ADMINISTRADOR`.
4. Inicia sesion con el nuevo usuario.
5. Desactiva el administrador inicial si ya no se usara.

## Flujo principal

1. Entra a `Ubicaciones`.
2. Crea o selecciona una ubicacion raiz.
3. Dentro del arbol puedes crear sububicaciones, laboratorios o equipos.
4. Los equipos pertenecen a una ubicacion.
5. Los laboratorios consumen equipos de su misma ubicacion para mostrarlos en la
   escena 3D.
6. Desde la gestion de un equipo puedes subir o cambiar su modelo 3D.
7. Desde la gestion de un laboratorio puedes agregar/quitar equipos, ver la
   previsualizacion y entrar al visor 3D.
8. En `Alertas` se revisan alertas pendientes, pospuestas o resueltas.

## Notas

- Los modelos 3D se guardan desde el backend, no en el frontend.
- En produccion se recomienda cambiar siempre las credenciales iniciales.
- Para despliegue en Proxmox revisa `DEPLOY_PROXMOX.md`.
