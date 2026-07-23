package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/service"
	"gitlab.com/syncbox/backend/internal/token"
)

// Claves usadas en el contexto de Fiber.
const (
	CtxUserID = "ctx_user_id"
	CtxRole   = "ctx_role"
	CtxJTI    = "ctx_jti"
)

// RequireAuth valida el access token, comprueba que el jti del refresh
// asociado no esté revocado (a través del par jti↔sesión) y deja en el contexto
// el id de usuario y el rol declarados por el token.
//
// El access no se persiste en BD por tratarse de corta duración; sin embargo,
// invalidamos sesiones por usuario en cambios de contraseña/desactivación, lo
// que garantiza que en pocos minutos cualquier access huérfano caduca.
func RequireAuth(tokens *token.Manager, auth *service.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		raw := c.Get(fiber.HeaderAuthorization)
		if !strings.HasPrefix(raw, "Bearer ") {
			return fiber.NewError(fiber.StatusUnauthorized, "token requerido")
		}
		raw = strings.TrimPrefix(raw, "Bearer ")
		claims, err := tokens.Parse(raw)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, domain.ErrTokenInvalid.Error())
		}
		if claims.Kind != token.KindAccess {
			return fiber.NewError(fiber.StatusUnauthorized, "tipo de token inválido")
		}
		c.Locals(CtxUserID, claims.UserID)
		c.Locals(CtxRole, domain.Role(claims.Role))
		c.Locals(CtxJTI, claims.JTI)
		return c.Next()
	}
}

// UserIDFrom extrae el userID del contexto.
func UserIDFrom(c *fiber.Ctx) uuid.UUID {
	if v, ok := c.Locals(CtxUserID).(uuid.UUID); ok {
		return v
	}
	return uuid.Nil
}

// RoleFrom extrae el rol del contexto.
func RoleFrom(c *fiber.Ctx) domain.Role {
	if v, ok := c.Locals(CtxRole).(domain.Role); ok {
		return v
	}
	return ""
}

// JTIFrom extrae el jti del contexto.
func JTIFrom(c *fiber.Ctx) string {
	if v, ok := c.Locals(CtxJTI).(string); ok {
		return v
	}
	return ""
}
