package middleware

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"

	"gitlab.com/syncbox/backend/internal/domain"
)

// ErrorHandler centraliza la traducción de errores de dominio a HTTP. No filtra
// detalles internos: solo exposes mensajes seguros.
func ErrorHandler(c *fiber.Ctx, err error) error {
	// Errores propios de Fiber (NewError) se respetan.
	var fe *fiber.Error
	if errors.As(err, &fe) {
		return c.Status(fe.Code).JSON(fiber.Map{"error": fe.Message})
	}

	switch {
	case errors.Is(err, domain.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidInput):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrInvalidCredential):
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "credenciales inválidas"})
	case errors.Is(err, domain.ErrAccountBlocked):
		return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrAccountInactive),
		errors.Is(err, domain.ErrTokenInvalid),
		errors.Is(err, domain.ErrTokenRevoked),
		errors.Is(err, domain.ErrUnauthorized):
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrForbidden), errors.Is(err, domain.ErrLastAdmin):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrConflict):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrModelEmpty),
		errors.Is(err, domain.ErrModelTooLarge),
		errors.Is(err, domain.ErrModelFormat),
		errors.Is(err, domain.ErrNodoTipoInvalido),
		errors.Is(err, domain.ErrConfirmRequerida):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, domain.ErrNodoCiclo),
		errors.Is(err, domain.ErrNodoTienHijos),
		errors.Is(err, domain.ErrModeloEnUso):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	}

	log.Error().Err(err).Str("path", c.Path()).Msg("error no manejado")
	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "error interno"})
}
