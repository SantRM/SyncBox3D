package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/repository"
)

// RequireRole permite el paso solo si el rol del usuario, **revalidado contra
// la base de datos**, está dentro de la lista permitida.
//
// La revalidación contra BD evita que un token siga siendo válido después de
// que un administrador haya degradado al usuario.
func RequireRole(users *repository.UserRepo, allowed ...domain.Role) fiber.Handler {
	allow := make(map[domain.Role]struct{}, len(allowed))
	for _, r := range allowed {
		allow[r] = struct{}{}
	}
	return func(c *fiber.Ctx) error {
		uid := UserIDFrom(c)
		if uid == uuid.Nil {
			return fiber.NewError(fiber.StatusUnauthorized, "no autenticado")
		}
		u, err := users.FindByID(c.Context(), uid)
		if err != nil || !u.Activo {
			return fiber.NewError(fiber.StatusUnauthorized, "cuenta inválida")
		}
		if _, ok := allow[u.Rol]; !ok {
			return fiber.NewError(fiber.StatusForbidden, domain.ErrForbidden.Error())
		}
		// Refrescar el rol en el contexto por si cambió.
		c.Locals(CtxRole, u.Rol)
		return c.Next()
	}
}
