package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/http/middleware"
	"gitlab.com/syncbox/backend/internal/service"
)

// NodoHandler agrupa endpoints de /nodos.
type NodoHandler struct{ svc *service.NodoService }

// NewNodoHandler construye el handler.
func NewNodoHandler(svc *service.NodoService) *NodoHandler { return &NodoHandler{svc: svc} }

// List: GET /nodos[?parent_id=]
func (h *NodoHandler) List(c *fiber.Ctx) error {
	if v := c.Query("parent_id"); v != "" {
		pid, err := uuid.Parse(v)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "parent_id inválido")
		}
		out, err := h.svc.Children(c.Context(), pid)
		if err != nil {
			return err
		}
		return c.JSON(out)
	}
	out, err := h.svc.Roots(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Get: GET /nodos/:id
func (h *NodoHandler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	n, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(n)
}

// Children: GET /nodos/:id/children
func (h *NodoHandler) Children(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	out, err := h.svc.Children(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Subtree: GET /nodos/:id/subtree
func (h *NodoHandler) Subtree(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	out, err := h.svc.Subtree(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Ancestors: GET /nodos/:id/ancestors
func (h *NodoHandler) Ancestors(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	out, err := h.svc.Ancestors(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Create: POST /nodos
func (h *NodoHandler) Create(c *fiber.Ctx) error {
	var in service.NodoCreateInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON inválido")
	}
	n, err := h.svc.Create(c.Context(), middleware.UserIDFrom(c), in)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(n)
}

// Update: PATCH /nodos/:id
func (h *NodoHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	var in service.NodoUpdateInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON inválido")
	}
	n, err := h.svc.Update(c.Context(), middleware.UserIDFrom(c), id, in)
	if err != nil {
		return err
	}
	return c.JSON(n)
}

type moveReq struct {
	NewParentID *uuid.UUID `json:"new_parent_id"`
}

// Move: POST /nodos/:id/move
func (h *NodoHandler) Move(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	var in moveReq
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON inválido")
	}
	if err := h.svc.Move(c.Context(), middleware.UserIDFrom(c), id, in.NewParentID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type deleteReq struct {
	Confirm             string     `json:"confirm"`
	ReplacementParentID *uuid.UUID `json:"replacement_parent_id,omitempty"`
	Promote             bool       `json:"promote,omitempty"`
}

// Delete: DELETE /nodos/:id
func (h *NodoHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	var in deleteReq
	_ = c.BodyParser(&in)
	if err := h.svc.Delete(c.Context(), middleware.UserIDFrom(c), id, service.DeleteOpts{
		Confirm:             in.Confirm,
		ReplacementParentID: in.ReplacementParentID,
		Promote:             in.Promote,
	}); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
