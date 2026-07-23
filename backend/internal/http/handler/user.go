package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/http/middleware"
	"gitlab.com/syncbox/backend/internal/service"
)

// UserHandler agrupa los endpoints de gestión de usuarios (solo Admin).
type UserHandler struct{ svc *service.UserService }

// NewUserHandler construye el handler.
func NewUserHandler(svc *service.UserService) *UserHandler { return &UserHandler{svc: svc} }

// List: GET /usuarios
func (h *UserHandler) List(c *fiber.Ctx) error {
	us, err := h.svc.List(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(us)
}

// Get: GET /usuarios/:id
func (h *UserHandler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	u, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(u)
}

// Create: POST /usuarios
func (h *UserHandler) Create(c *fiber.Ctx) error {
	var in service.CreateInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON inválido")
	}
	u, err := h.svc.Create(c.Context(), middleware.UserIDFrom(c), in)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(u)
}

// Update: PATCH /usuarios/:id
func (h *UserHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	var in service.UpdateInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON inválido")
	}
	u, err := h.svc.Update(c.Context(), middleware.UserIDFrom(c), id, in)
	if err != nil {
		return err
	}
	return c.JSON(u)
}

// Deactivate: DELETE /usuarios/:id (soft).
func (h *UserHandler) Deactivate(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	if err := h.svc.Deactivate(c.Context(), middleware.UserIDFrom(c), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
