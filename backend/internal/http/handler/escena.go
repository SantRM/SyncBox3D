package handler

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/http/middleware"
	"gitlab.com/syncbox/backend/internal/repository"
	"gitlab.com/syncbox/backend/internal/service"
)

// EscenaHandler agrupa endpoints de laboratorios (escenas) e instancias.
type EscenaHandler struct{ svc *service.EscenaService }

// NewEscenaHandler construye el handler.
func NewEscenaHandler(svc *service.EscenaService) *EscenaHandler { return &EscenaHandler{svc: svc} }

// List: GET /escenas?activas=true
func (h *EscenaHandler) List(c *fiber.Ctx) error {
	soloActivas := c.Query("activas") == "true"
	out, err := h.svc.ListEscenas(c.Context(), soloActivas)
	if err != nil {
		return err
	}
	if out == nil {
		out = []domain.Escena{}
	}
	return c.JSON(out)
}

// Get: GET /escenas/:id  (incluye instancias)
func (h *EscenaHandler) Get(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	det, err := h.svc.GetEscena(c.Context(), id)
	if err != nil {
		return err
	}
	return c.JSON(det)
}

// Create: POST /escenas
func (h *EscenaHandler) Create(c *fiber.Ctx) error {
	var in domain.Escena
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON inválido")
	}
	if err := h.svc.CreateEscena(c.Context(), middleware.UserIDFrom(c), &in); err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(in)
}

// Update: PATCH /escenas/:id
func (h *EscenaHandler) Update(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	var in struct {
		Nombre      *string `json:"nombre,omitempty"`
		Descripcion *string `json:"descripcion,omitempty"`
		Activo      *bool   `json:"activo,omitempty"`
		NodoID      *string `json:"nodo_id,omitempty"`
	}
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON inválido")
	}
	var nodoID *uuid.UUID
	if in.NodoID != nil && *in.NodoID != "" {
		parsed, err := uuid.Parse(*in.NodoID)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "nodo_id invÃ¡lido")
		}
		nodoID = &parsed
	}
	if err := h.svc.UpdateEscena(c.Context(), middleware.UserIDFrom(c), id,
		in.Nombre, in.Descripcion, in.Activo, nodoID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// UpdateLighting: PATCH /escenas/:id/iluminacion
func (h *EscenaHandler) UpdateLighting(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invÃ¡lido")
	}
	var in domain.EscenaLight
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON invÃ¡lido")
	}
	light, err := h.svc.UpdateLighting(c.Context(), middleware.UserIDFrom(c), id, in)
	if err != nil {
		return err
	}
	return c.JSON(light)
}

// StartSesion: POST /escenas/:id/sesiones
func (h *EscenaHandler) StartSesion(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	sesion, err := h.svc.StartLabSesion(c.Context(), middleware.UserIDFrom(c), id)
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(sesion)
}

// CloseSesion: POST /escenas/:id/sesiones/:sid/cerrar
func (h *EscenaHandler) CloseSesion(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	sid, err := uuid.Parse(c.Params("sid"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "sid invalido")
	}
	var in struct {
		Motivo string `json:"motivo"`
	}
	if len(c.Body()) > 0 {
		if err := c.BodyParser(&in); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "JSON invalido")
		}
	}
	sesion, err := h.svc.CloseLabSesion(c.Context(), middleware.UserIDFrom(c), id, sid, in.Motivo)
	if err != nil {
		return err
	}
	return c.JSON(sesion)
}

// Auditoria: GET /escenas/:id/auditoria
func (h *EscenaHandler) Auditoria(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	desde, err := parseAuditTimeParam(c.Query("desde"), false)
	if err != nil {
		return err
	}
	hasta, err := parseAuditTimeParam(c.Query("hasta"), true)
	if err != nil {
		return err
	}
	out, err := h.svc.ListLabAudit(c.Context(), id, repository.LabAuditFilters{
		Search: c.Query("q"),
		Desde:  desde,
		Hasta:  hasta,
		Estado: c.Query("estado"),
		Limit:  c.QueryInt("limit", 80),
		Offset: c.QueryInt("offset", 0),
	})
	if err != nil {
		return err
	}
	return c.JSON(out)
}

// Delete: DELETE /escenas/:id
func (h *EscenaHandler) Delete(c *fiber.Ctx) error {
	id, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	if err := h.svc.DeleteEscena(c.Context(), middleware.UserIDFrom(c), id); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// --- Instancias --------------------------------------------------------------

// AddInstancia: POST /escenas/:id/instancias
func (h *EscenaHandler) AddInstancia(c *fiber.Ctx) error {
	escenaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	var in struct {
		EquipoID    string  `json:"equipo_id"`
		LabSesionID *string `json:"lab_sesion_id,omitempty"`
		PosX        float64 `json:"pos_x"`
		PosY        float64 `json:"pos_y"`
		PosZ        float64 `json:"pos_z"`
		Escala      float64 `json:"escala"`
		RotX        float64 `json:"rot_x"`
		RotY        float64 `json:"rot_y"`
		RotZ        float64 `json:"rot_z"`
	}
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON inválido")
	}
	equipoID, err := uuid.Parse(in.EquipoID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "equipo_id inválido")
	}
	sesionID, err := parseOptionalUUID(in.LabSesionID, "lab_sesion_id")
	if err != nil {
		return err
	}
	inst, err := h.svc.AddInstancia(c.Context(), middleware.UserIDFrom(c), escenaID, service.AddInstanciaInput{
		EquipoID:    equipoID,
		LabSesionID: sesionID,
		PosX:        in.PosX, PosY: in.PosY, PosZ: in.PosZ,
		Escala: in.Escala,
		RotX:   in.RotX, RotY: in.RotY, RotZ: in.RotZ,
	})
	if err != nil {
		return err
	}
	return c.Status(fiber.StatusCreated).JSON(inst)
}

// UpdateInstancia: PATCH /escenas/:id/instancias/:iid
func (h *EscenaHandler) UpdateInstancia(c *fiber.Ctx) error {
	escenaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	instID, err := uuid.Parse(c.Params("iid"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "iid invalido")
	}
	var in struct {
		LabSesionID *string  `json:"lab_sesion_id,omitempty"`
		PosX        *float64 `json:"pos_x,omitempty"`
		PosY        *float64 `json:"pos_y,omitempty"`
		PosZ        *float64 `json:"pos_z,omitempty"`
		Escala      *float64 `json:"escala,omitempty"`
		RotX        *float64 `json:"rot_x,omitempty"`
		RotY        *float64 `json:"rot_y,omitempty"`
		RotZ        *float64 `json:"rot_z,omitempty"`
	}
	if err := c.BodyParser(&in); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JSON invalido")
	}
	sesionID, err := parseOptionalUUID(in.LabSesionID, "lab_sesion_id")
	if err != nil {
		return err
	}
	inst, err := h.svc.UpdateInstancia(c.Context(), middleware.UserIDFrom(c), escenaID, instID, sesionID,
		in.PosX, in.PosY, in.PosZ, in.Escala, in.RotX, in.RotY, in.RotZ)
	if err != nil {
		return err
	}
	return c.JSON(inst)
}

// RestoreInstancia: POST /escenas/:id/instancias/:iid/restore
func (h *EscenaHandler) RestoreInstancia(c *fiber.Ctx) error {
	escenaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	instID, err := uuid.Parse(c.Params("iid"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "iid invalido")
	}
	sesionID, err := parseLabSesionIDBody(c)
	if err != nil {
		return err
	}
	inst, err := h.svc.RestoreInstancia(c.Context(), middleware.UserIDFrom(c), escenaID, instID, sesionID)
	if err != nil {
		return err
	}
	return c.JSON(inst)
}

// RestoreInstanciaFromLastSession: POST /escenas/:id/instancias/:iid/restore-session
func (h *EscenaHandler) RestoreInstanciaFromLastSession(c *fiber.Ctx) error {
	escenaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id invalido")
	}
	instID, err := uuid.Parse(c.Params("iid"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "iid invalido")
	}
	sesionID, err := parseLabSesionIDBody(c)
	if err != nil {
		return err
	}
	inst, err := h.svc.RestoreInstanciaFromLastSession(c.Context(), middleware.UserIDFrom(c), escenaID, instID, sesionID)
	if err != nil {
		return err
	}
	return c.JSON(inst)
}

// RemoveInstancia: DELETE /escenas/:id/instancias/:iid
func (h *EscenaHandler) RemoveInstancia(c *fiber.Ctx) error {
	escenaID, err := uuid.Parse(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "id inválido")
	}
	instID, err := uuid.Parse(c.Params("iid"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "iid inválido")
	}
	if err := h.svc.RemoveInstancia(c.Context(), middleware.UserIDFrom(c), escenaID, instID); err != nil {
		return err
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func parseOptionalUUID(raw *string, field string) (*uuid.UUID, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(*raw)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, field+" invalido")
	}
	return &id, nil
}

func parseLabSesionIDBody(c *fiber.Ctx) (*uuid.UUID, error) {
	if len(c.Body()) == 0 {
		return nil, nil
	}
	var in struct {
		LabSesionID *string `json:"lab_sesion_id,omitempty"`
	}
	if err := c.BodyParser(&in); err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "JSON invalido")
	}
	return parseOptionalUUID(in.LabSesionID, "lab_sesion_id")
}

func parseAuditTimeParam(raw string, endOfDay bool) (*time.Time, error) {
	if raw == "" {
		return nil, nil
	}
	if t, err := time.Parse("2006-01-02", raw); err == nil {
		if endOfDay {
			t = t.AddDate(0, 0, 1)
		}
		return &t, nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "fecha invalida")
	}
	return &t, nil
}
