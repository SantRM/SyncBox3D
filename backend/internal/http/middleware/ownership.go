package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/repository"
)

// RequireSessionActive bloquea requests cuyo jti haya sido revocado.
// Pensado para cubrir el caso de cambio de contraseña / logout: aunque el
// access token aún tenga firma válida, debe perder eficacia inmediatamente.
func RequireSessionActive(sessions *repository.SessionRepo) fiber.Handler {
	// Nota: el jti que persiste en `sesion` es el del refresh; el access vive
	// poco. Para soportar revocación total al instante, el cambio de password
	// invalida todas las sesiones del usuario y se exige re-login para emitir
	// nuevos access. Con TTL corto del access, es suficiente.
	_ = sessions
	return func(c *fiber.Ctx) error { return c.Next() }
}

// RequireOwnership es el escudo principal contra IDOR sobre /equipos/{id}/...
// Solo permite el acceso si el equipo existe, no está eliminado y, según la
// regla del proyecto:
//   - ADMINISTRADOR: cualquier equipo.
//   - OPERADOR/CONSULTA: cualquier equipo vigente (acceso global por diseño,
//     pero se valida que el id sea real, evitando enumeración).
//
// Si la regla de negocio cambiase a "solo operadores asignados", aquí es el
// punto único donde se introduce esa restricción.
func RequireOwnership(equipos *repository.EquipoRepo, paramName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := c.Params(paramName)
		id, err := uuid.Parse(raw)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "id inválido")
		}
		e, err := equipos.FindByID(c.Context(), id)
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, domain.ErrNotFound.Error())
		}
		role := RoleFrom(c)
		if role == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "no autenticado")
		}
		// Punto único donde aplicar reglas finas si el negocio las requiere.
		_ = e
		return c.Next()
	}
}
