package handler

import (
	"mime/multipart"
	"path/filepath"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/http/middleware"
	"gitlab.com/syncbox/backend/internal/service"
)

// Modelo3DHandler agrupa endpoints de /modelos3d.
type Modelo3DHandler struct{ svc *service.Modelo3DService }

// NewModelo3DHandler construye el handler.
func NewModelo3DHandler(svc *service.Modelo3DService) *Modelo3DHandler {
	return &Modelo3DHandler{svc: svc}
}

// List: GET /modelos3d
func (h *Modelo3DHandler) List(c *fiber.Ctx) error {
	out, err := h.svc.List(c.Context(), c.Query("q"))
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Get: GET /modelos3d/:id
func (h *Modelo3DHandler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	m, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(m)
}

// File: GET /modelos3d/:id/file
func (h *Modelo3DHandler) File(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	m, err := h.svc.Get(c.Context(), id)
	if err != nil {
		return err
	}
	path, err := h.svc.FilePath(m)
	if err != nil {
		return err
	}
	c.Set(fiber.HeaderContentType, m.Mime)
	c.Set(fiber.HeaderContentDisposition, `inline; filename="`+filepath.Base(m.StorageURI)+`"`)
	c.Set(fiber.HeaderCacheControl, "private, max-age=3600")
	return c.SendFile(path, false)
}

// Upload: POST /modelos3d (multipart: file, assets?, nombre?, descripcion?)
func (h *Modelo3DHandler) Upload(c *fiber.Ctx) error {
	fh, err := c.FormFile("file")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "archivo requerido")
	}
	var assets []*multipart.FileHeader
	var assetPaths []string
	if form, err := c.MultipartForm(); err == nil && form != nil {
		assets = form.File["assets"]
		assetPaths = form.Value["asset_path"]
	}
	nombre := c.FormValue("nombre")
	descripcion := c.FormValue("descripcion")
	m, err := h.svc.Upload(c.Context(), middleware.UserIDFrom(c), nombre, descripcion, fh, c.FormValue("file_path"), assets, assetPaths)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(m)
}

type modelUpdateReq struct {
	Nombre      *string `json:"nombre,omitempty"`
	Descripcion *string `json:"descripcion,omitempty"`
}

// Update: PATCH /modelos3d/:id
func (h *Modelo3DHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	var in modelUpdateReq
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON inválido")
	}
	m, err := h.svc.Update(c.Context(), middleware.UserIDFrom(c), id, in.Nombre, in.Descripcion)
	if err != nil {
		return err
	}
	return c.JSON(m)
}

// Delete: DELETE /modelos3d/:id
func (h *Modelo3DHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	if err := h.svc.Delete(c.Context(), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}
