package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/http/middleware"
	"gitlab.com/syncbox/backend/internal/repository"
	"gitlab.com/syncbox/backend/internal/service"
)

// EquipoHandler agrupa endpoints de /equipos.
type EquipoHandler struct{ svc *service.EquipoService }

// NewEquipoHandler construye el handler.
func NewEquipoHandler(svc *service.EquipoService) *EquipoHandler { return &EquipoHandler{svc: svc} }

// List: GET /equipos
func (h *EquipoHandler) List(c *fiber.Ctx) error {
	in := service.ListInput{Search: c.Query("q")}
	if v := c.Query("categoria_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "categoria_id invalido")
		}
		in.CategoriaID = &id
	}
	if v := c.Query("estado_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "estado_id invalido")
		}
		in.EstadoID = &id
	}
	if v := c.Query("nodo_id"); v != "" {
		id, err := uuid.Parse(v)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "nodo_id invalido")
		}
		in.NodoID = &id
	}
	in.Limit, _ = strconv.Atoi(c.Query("limit"))
	in.Offset, _ = strconv.Atoi(c.Query("offset"))
	es, err := h.svc.List(c.Context(), in)
	if err != nil {
		return err
	}
	return c.JSON(es)
}

// Get: GET /equipos/:id
func (h *EquipoHandler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	e, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(e)
}

// Create: POST /equipos
func (h *EquipoHandler) Create(c *fiber.Ctx) error {
	var in service.EquipoCreateInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON invalido")
	}
	e, err := h.svc.Create(c.Context(), middleware.UserIDFrom(c), in)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(e)
}

// Update: PATCH /equipos/:id
func (h *EquipoHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	var in service.EquipoUpdateInput
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON invalido")
	}
	e, err := h.svc.Update(c.Context(), middleware.UserIDFrom(c), id, in)
	if err != nil {
		return err
	}
	return c.JSON(e)
}

// Delete: DELETE /equipos/:id (soft).
func (h *EquipoHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	if err := h.svc.Delete(c.Context(), middleware.UserIDFrom(c), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

type setModelo3DReq struct {
	Modelo3DID *uuid.UUID `json:"modelo_3d_id"`
}

// SetModelo3D: PATCH /equipos/:id/modelo3d
func (h *EquipoHandler) SetModelo3D(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	var in setModelo3DReq
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON invalido")
	}
	e, err := h.svc.SetModelo3D(c.Context(), middleware.UserIDFrom(c), id, in.Modelo3DID)
	if err != nil {
		return err
	}
	return c.JSON(e)
}

type changeStateReq struct {
	EstadoID uuid.UUID `json:"estado_id"`
	Motivo   string    `json:"motivo"`
}

// ChangeState: PATCH /equipos/:id/estado
func (h *EquipoHandler) ChangeState(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	var in changeStateReq
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON invalido")
	}
	if in.EstadoID == uuid.Nil {
		return fiber.NewError(fiber.StatusBadRequest, "estado_id requerido")
	}
	if err := h.svc.ChangeState(c.Context(), middleware.UserIDFrom(c), id, in.EstadoID, in.Motivo); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Historial: GET /equipos/:id/historial
func (h *EquipoHandler) Historial(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	estados, err := h.svc.ListEstadoHistorial(c.Context(), id)
	if err != nil {
		return err
	}
	cambios, err := h.svc.ListCambios(c.Context(), id)
	if err != nil {
		return err
	}
	if estados == nil {
		estados = []repository.EstadoHistorialEntry{}
	}
	if cambios == nil {
		cambios = []repository.CambioEntry{}
	}
	return c.JSON(fiber.Map{"estados": estados, "cambios": cambios})
}

// GetFicha: GET /equipos/:id/ficha
func (h *EquipoHandler) GetFicha(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	f, err := h.svc.GetFicha(c.Context(), id)
	if err != nil {
		return err
	}
	if f == nil {
		return c.JSON(nil)
	}
	return c.JSON(f)
}

// UpsertFicha: PUT /equipos/:id/ficha
func (h *EquipoHandler) UpsertFicha(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	var f repository.FichaTecnica
	if err := c.BodyParser(&f); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON invalido")
	}
	out, err := h.svc.UpsertFicha(c.Context(), middleware.UserIDFrom(c), id, &f)
	if err != nil {
		return err
	}
	return c.JSON(out)
}
