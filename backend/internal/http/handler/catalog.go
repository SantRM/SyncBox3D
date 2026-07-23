package handler

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/http/middleware"
	"gitlab.com/syncbox/backend/internal/service"
)

// CatalogHandler agrupa endpoints de categorías y estados.
type CatalogHandler struct{ svc *service.CatalogService }

// NewCatalogHandler construye el handler.
func NewCatalogHandler(svc *service.CatalogService) *CatalogHandler { return &CatalogHandler{svc: svc} }

// ListCategorias: GET /categorias
func (h *CatalogHandler) ListCategorias(c *fiber.Ctx) error {
	soloActivas := c.Query("activas") == "true"
	cs, err := h.svc.ListCategorias(c.Context(), soloActivas)
	if err != nil {
		return err
	}
	return c.JSON(cs)
}

// CreateCategoria: POST /categorias (Admin)
func (h *CatalogHandler) CreateCategoria(c *fiber.Ctx) error {
	var in domain.Categoria
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON inválido")
	}
	if err := h.svc.CreateCategoria(c.Context(), middleware.UserIDFrom(c), &in); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(in)
}

// UpdateCategoria: PATCH /categorias/:id (Admin)
func (h *CatalogHandler) UpdateCategoria(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	var in struct {
		Nombre      *string `json:"nombre,omitempty"`
		Descripcion *string `json:"descripcion,omitempty"`
		Activo      *bool   `json:"activo,omitempty"`
	}
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON inválido")
	}
	if err := h.svc.UpdateCategoria(c.Context(), middleware.UserIDFrom(c), id, in.Nombre, in.Descripcion, in.Activo); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// ListEstados: GET /estados
func (h *CatalogHandler) ListEstados(c *fiber.Ctx) error {
	es, err := h.svc.ListEstados(c.Context())
	if err != nil {
		return err
	}
	return c.JSON(es)
}
