package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/http/middleware"
	"gitlab.com/syncbox/backend/internal/repository"
	"gitlab.com/syncbox/backend/internal/service"
)

// AuthHandler agrupa los endpoints de /auth.
type AuthHandler struct {
	svc   *service.AuthService
	users *repository.UserRepo
}

// NewAuthHandler construye el handler.
func NewAuthHandler(svc *service.AuthService, users *repository.UserRepo) *AuthHandler {
	return &AuthHandler{svc: svc, users: users}
}

type loginReq struct {
	Correo   string `json:"correo"`
	Password string `json:"password"`
}

// Login: POST /auth/login
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var in loginReq
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON inválido")
	}
	res, err := h.svc.Login(c.Context(), in.Correo, in.Password, c.IP())
	if err != nil {
		return err
	}
	return c.JSON(res)
}

type refreshReq struct {
	RefreshToken string `json:"refresh_token"`
}

// Refresh: POST /auth/refresh
func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
	var in refreshReq
	if err := c.BodyParser(&in); err != nil || in.RefreshToken == "" {
		return fiber.NewError(fiber.StatusBadRequest, "refresh_token requerido")
	}
	res, err := h.svc.Refresh(c.Context(), in.RefreshToken)
	if err != nil {
		return err
	}
	return c.JSON(res)
}

// Logout: POST /auth/logout — revoca el jti actual.
// Nota: el jti que se persiste es el del refresh; al hacer logout, también
// invalidamos cualquier refresh asociado al user para corte limpio.
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	uid := middleware.UserIDFrom(c)
	// Revocamos todas las sesiones del usuario; corte limpio.
	if err := h.svc.IsValidUser(c.Context(), uid); err != nil {
		return err
	}
	if err := h.svc.LogoutAll(c.Context(), uid); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type changePwdReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// ChangePassword: POST /auth/password
func (h *AuthHandler) ChangePassword(c *fiber.Ctx) error {
	var in changePwdReq
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON inválido")
	}
	uid := middleware.UserIDFrom(c)
	if err := h.svc.ChangePassword(c.Context(), uid, in.OldPassword, in.NewPassword); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Me: GET /me
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	uid := middleware.UserIDFrom(c)
	u, err := h.users.FindByID(c.Context(), uid)
	if err != nil {
		return err
	}
	if !u.Activo {
		// El access token sigue válido por su TTL pero la cuenta fue
		// desactivada: cerramos cualquier acción con 401 inmediato.
		return fiber.NewError(fiber.StatusUnauthorized, "cuenta inactiva")
	}
	return c.JSON(u.ToPublic())
}

type updatePreferencesReq struct {
	Idioma domain.Locale `json:"idioma"`
}

// UpdatePreferences: PATCH /me/preferencias
func (h *AuthHandler) UpdatePreferences(c *fiber.Ctx) error {
	var in updatePreferencesReq
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON invalido")
	}
	locale := domain.Locale(strings.ToLower(strings.TrimSpace(string(in.Idioma))))
	if !locale.Valid() {
		return fiber.NewError(fiber.StatusBadRequest, "idioma invalido")
	}

	uid := middleware.UserIDFrom(c)
	u, err := h.users.FindByID(c.Context(), uid)
	if err != nil {
		return err
	}
	if !u.Activo {
		return fiber.NewError(fiber.StatusUnauthorized, "cuenta inactiva")
	}
	if err := h.users.UpdatePreferredLanguage(c.Context(), uid, locale); err != nil {
		return err
	}
	u.IdiomaPreferido = locale
	return c.JSON(u.ToPublic())
}
