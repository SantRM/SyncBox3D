package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/http/middleware"
	"gitlab.com/syncbox/backend/internal/service"
)

// AlertaHandler agrupa endpoints de /alertas.
type AlertaHandler struct{ svc *service.AlertaService }

// NewAlertaHandler construye el handler.
func NewAlertaHandler(svc *service.AlertaService) *AlertaHandler { return &AlertaHandler{svc: svc} }

// List: GET /alertas?estado=pendiente|resuelta&q=...
func (h *AlertaHandler) List(c *fiber.Ctx) error {
	as, err := h.svc.ListEventos(
		c.Context(),
		c.Query("estado"),
		c.Query("q"),
		c.QueryInt("limit", 100),
		c.QueryInt("offset", 0),
	)
	if err != nil {
		return err
	}
	return c.JSON(as)
}

// Pendientes: GET /alertas/pendientes?due=true
func (h *AlertaHandler) Pendientes(c *fiber.Ctx) error {
	as, err := h.svc.Pendientes(c.Context(), c.Query("due") == "true")
	if err != nil {
		return err
	}
	return c.JSON(as)
}

// Configuracion: GET /alertas/config
func (h *AlertaHandler) Configuracion(c *fiber.Ctx) error {
	cfg, err := h.svc.Configuracion(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(cfg)
}

// UpdateConfig: PATCH /alertas/config/:estado_id
func (h *AlertaHandler) UpdateConfig(c *fiber.Ctx) error {
	estadoID, err := uuid.Parse(c.Params("estado_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "estado_id invalido")
	}
	var in struct {
		DiasUmbral int  `json:"dias_umbral"`
		Activa     bool `json:"activa"`
	}
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON invalido")
	}
	cfg, err := h.svc.ActualizarConfig(c.Context(), estadoID, in.DiasUmbral, in.Activa)
	if err != nil {
		return err
	}
	return c.JSON(cfg)
}

// Resolver: POST /alertas/:id/resolver
func (h *AlertaHandler) Resolver(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	if err := h.svc.Resolver(c.Context(), id, middleware.UserIDFrom(c)); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// Posponer: POST /alertas/:id/posponer
func (h *AlertaHandler) Posponer(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	var in struct {
		Minutes int `json:"minutes"`
	}
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&in); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "JSON invalido")
		}
	}
	if err := h.svc.Posponer(c.Context(), id, middleware.UserIDFrom(c), in.Minutes); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// MarkVisto: POST /alertas/:id/visto
func (h *AlertaHandler) MarkVisto(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	if err := h.svc.MarcarVista(c.Context(), id, middleware.UserIDFrom(c)); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
