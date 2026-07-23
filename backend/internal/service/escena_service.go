package service

import (
	"context"
	"math"
	"strings"

	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/repository"
)

// EscenaService implementa las reglas de negocio sobre escenas y sus
// instancias (laboratorios del Nivel 2).
type EscenaService struct {
	escenas    *repository.EscenaRepo
	equipos    *repository.EquipoRepo
	categorias *repository.CategoriaRepo
	nodos      *repository.NodoRepo
}

// NewEscenaService construye el servicio.
func NewEscenaService(e *repository.EscenaRepo, eq *repository.EquipoRepo, c *repository.CategoriaRepo, n *repository.NodoRepo) *EscenaService {
	return &EscenaService{escenas: e, equipos: eq, categorias: c, nodos: n}
}

// ListEscenas lista escenas no borradas.
func (s *EscenaService) ListEscenas(ctx context.Context, soloActivas bool) ([]domain.Escena, error) {
	return s.escenas.List(ctx, soloActivas)
}

// GetEscena devuelve la escena junto con sus instancias.
func (s *EscenaService) GetEscena(ctx context.Context, id uuid.UUID) (*domain.EscenaDetail, error) {
	esc, err := s.escenas.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	insts, err := s.escenas.ListInstancias(ctx, id)
	if err != nil {
		return nil, err
	}
	if insts == nil {
		insts = []domain.EscenaInstancia{}
	}
	return &domain.EscenaDetail{Escena: *esc, Instancias: insts}, nil
}

// CreateEscena crea una escena vacía.
func (s *EscenaService) CreateEscena(ctx context.Context, actor uuid.UUID, e *domain.Escena) error {
	e.Nombre = strings.TrimSpace(e.Nombre)
	e.Descripcion = strings.TrimSpace(e.Descripcion)
	if e.Nombre == "" {
		return domain.ErrInvalidInput
	}
	e.Activo = true
	e.Iluminacion = defaultSceneLight()
	return s.escenas.Create(ctx, e, actor)
}

// UpdateEscena modifica metadata.
func (s *EscenaService) UpdateEscena(ctx context.Context, actor, id uuid.UUID, nombre, descripcion *string, activo *bool, nodoID *uuid.UUID) error {
	if nombre != nil {
		v := strings.TrimSpace(*nombre)
		if v == "" {
			return domain.ErrInvalidInput
		}
		nombre = &v
	}
	if descripcion != nil {
		v := strings.TrimSpace(*descripcion)
		descripcion = &v
	}
	return s.escenas.Update(ctx, id, nombre, descripcion, activo, nodoID, actor)
}

// UpdateLighting modifica el foco propio del laboratorio.
func (s *EscenaService) UpdateLighting(ctx context.Context, actor, id uuid.UUID, light domain.EscenaLight) (*domain.EscenaLight, error) {
	if _, err := s.escenas.FindByID(ctx, id); err != nil {
		return nil, err
	}
	light.Color = strings.TrimSpace(light.Color)
	if light.Color == "" {
		light.Color = "#fff4d6"
	}
	if !validHexColor(light.Color) {
		return nil, domain.ErrInvalidInput
	}
	if light.Angulo <= 0 {
		light.Angulo = 0.55
	}
	if light.Distancia <= 0 {
		light.Distancia = 30
	}
	if light.Penumbra < 0 || light.Penumbra > 1 || light.Angulo > math.Pi/2 || light.Intensidad < 0 {
		return nil, domain.ErrInvalidInput
	}
	if !isFinite(
		light.Intensidad,
		light.PosX, light.PosY, light.PosZ,
		light.TargetX, light.TargetY, light.TargetZ,
		light.Angulo, light.Penumbra, light.Distancia,
	) {
		return nil, domain.ErrInvalidInput
	}
	if err := s.escenas.UpdateLighting(ctx, id, light, actor); err != nil {
		return nil, err
	}
	return &light, nil
}

// StartLabSesion registra una entrada al modo interactivo de un laboratorio.
func (s *EscenaService) StartLabSesion(ctx context.Context, actor, escenaID uuid.UUID) (*domain.LabSesion, error) {
	if _, err := s.escenas.FindByID(ctx, escenaID); err != nil {
		return nil, err
	}
	return s.escenas.CreateLabSesion(ctx, escenaID, actor)
}

// CloseLabSesion cierra una sesion de laboratorio si el cliente alcanza a
// avisar. Si no alcanza, ultima_actividad_at sigue representando su ultimo
// estado historico.
func (s *EscenaService) CloseLabSesion(ctx context.Context, actor, escenaID, sesionID uuid.UUID, motivo string) (*domain.LabSesion, error) {
	motivo = strings.TrimSpace(motivo)
	if motivo == "" {
		motivo = "manual"
	}
	if len(motivo) > 80 {
		motivo = motivo[:80]
	}
	return s.escenas.CloseLabSesion(ctx, escenaID, sesionID, actor, motivo)
}

// ListLabAudit expone la auditoria de transform por instancia y sesion.
func (s *EscenaService) ListLabAudit(ctx context.Context, escenaID uuid.UUID, filters repository.LabAuditFilters) (*domain.LabAuditResponse, error) {
	if _, err := s.escenas.FindByID(ctx, escenaID); err != nil {
		return nil, err
	}
	filters.Search = strings.TrimSpace(filters.Search)
	if filters.Estado != "abierta" && filters.Estado != "cerrada" {
		filters.Estado = ""
	}
	if filters.Limit <= 0 || filters.Limit > 200 {
		filters.Limit = 80
	}
	if filters.Offset < 0 {
		filters.Offset = 0
	}

	items, total, err := s.escenas.ListLabAudit(ctx, escenaID, filters)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.LabAuditEntry{}
	}
	return &domain.LabAuditResponse{
		Items:  items,
		Total:  total,
		Limit:  filters.Limit,
		Offset: filters.Offset,
	}, nil
}

// DeleteEscena soft-delete.
func (s *EscenaService) DeleteEscena(ctx context.Context, actor, id uuid.UUID) error {
	return s.escenas.SoftDelete(ctx, id, actor)
}

// AddInstanciaInput agrupa los inputs para añadir un equipo al laboratorio.
type AddInstanciaInput struct {
	EquipoID    uuid.UUID
	LabSesionID *uuid.UUID
	PosX        float64
	PosY        float64
	PosZ        float64
	Escala      float64
	RotX        float64
	RotY        float64
	RotZ        float64
}

// AddInstancia coloca un equipo en una escena. Toma un snapshot textual del
// equipo y de su categoría: si el equipo es modificado o borrado luego, la
// instancia conserva sus datos en el momento de la colocación.
func (s *EscenaService) AddInstancia(ctx context.Context, actor, escenaID uuid.UUID, in AddInstanciaInput) (*domain.EscenaInstancia, error) {
	esc, err := s.escenas.FindByID(ctx, escenaID)
	if err != nil {
		return nil, err
	}
	if in.Escala <= 0 {
		in.Escala = 1
	}
	if !isFinite(in.PosX, in.PosY, in.PosZ, in.Escala, in.RotX, in.RotY, in.RotZ) {
		return nil, domain.ErrInvalidInput
	}
	eq, err := s.equipos.FindByID(ctx, in.EquipoID)
	if err != nil {
		return nil, err
	}
	if err := s.validateEquipoDisponibleParaEscena(ctx, esc, eq); err != nil {
		return nil, err
	}
	catNombre := ""
	if cat, err := s.categorias.FindByID(ctx, eq.CategoriaID); err == nil && cat != nil {
		catNombre = cat.Nombre
	}
	id := eq.ID
	inst := &domain.EscenaInstancia{
		EscenaID:           escenaID,
		EquipoOrigenID:     &id,
		NombreSnapshot:     eq.Nombre,
		FabricanteSnapshot: eq.Fabricante,
		ModeloSnapshot:     eq.Modelo,
		CategoriaSnapshot:  catNombre,
		PosX:               in.PosX,
		PosY:               in.PosY,
		PosZ:               in.PosZ,
		Escala:             in.Escala,
		RotX:               in.RotX,
		RotY:               in.RotY,
		RotZ:               in.RotZ,
	}
	if err := s.escenas.CreateInstancia(ctx, inst, actor, in.LabSesionID); err != nil {
		return nil, err
	}
	return inst, nil
}

func (s *EscenaService) validateEquipoDisponibleParaEscena(ctx context.Context, esc *domain.Escena, eq *domain.Equipo) error {
	if esc.NodoID == nil || eq.NodoID == nil {
		return domain.ErrInvalidInput
	}
	labNode, err := s.nodos.FindByID(ctx, *esc.NodoID)
	if err != nil {
		return err
	}
	eqNode, err := s.nodos.FindByID(ctx, *eq.NodoID)
	if err != nil {
		return err
	}
	if labNode.Tipo != domain.NodoLaboratorio || eqNode.Tipo != domain.NodoEquipo {
		return domain.ErrInvalidInput
	}
	if labNode.ParentID == nil || eqNode.ParentID == nil || *labNode.ParentID != *eqNode.ParentID {
		return domain.ErrInvalidInput
	}
	return nil
}

// UpdateInstancia parche de transform. Si todos los punteros son nil, no hace nada.
func (s *EscenaService) UpdateInstancia(ctx context.Context, actor, escenaID, instanciaID uuid.UUID, sesionID *uuid.UUID, posX, posY, posZ, escala, rotX, rotY, rotZ *float64) (*domain.EscenaInstancia, error) {
	if escala != nil && *escala <= 0 {
		return nil, domain.ErrInvalidInput
	}
	if !allFinitePtrs(posX, posY, posZ, escala, rotX, rotY, rotZ) {
		return nil, domain.ErrInvalidInput
	}
	if err := s.escenas.UpdateInstanciaTransform(ctx, escenaID, instanciaID, posX, posY, posZ, escala, rotX, rotY, rotZ); err != nil {
		return nil, err
	}
	inst, err := s.escenas.FindInstancia(ctx, escenaID, instanciaID)
	if err != nil {
		return nil, err
	}
	if err := s.recordSessionSnapshot(ctx, actor, escenaID, sesionID, "transform", inst); err != nil {
		return nil, err
	}
	return inst, nil
}

// RestoreInstancia devuelve la instancia al transform base con el que Three.js
// carga un GLB en el laboratorio: posicion 0, escala 1 y rotacion 0.
func (s *EscenaService) RestoreInstancia(ctx context.Context, actor, escenaID, instanciaID uuid.UUID, sesionID *uuid.UUID) (*domain.EscenaInstancia, error) {
	zero := 0.0
	one := 1.0
	if err := s.escenas.UpdateInstanciaTransform(ctx, escenaID, instanciaID,
		&zero, &zero, &zero, &one,
		&zero, &zero, &zero); err != nil {
		return nil, err
	}
	inst, err := s.escenas.FindInstancia(ctx, escenaID, instanciaID)
	if err != nil {
		return nil, err
	}
	if err := s.recordSessionSnapshot(ctx, actor, escenaID, sesionID, "restore", inst); err != nil {
		return nil, err
	}
	return inst, nil
}

// RestoreInstanciaFromLastSession devuelve la instancia al ultimo transform
// guardado en una sesion anterior del laboratorio.
func (s *EscenaService) RestoreInstanciaFromLastSession(ctx context.Context, actor, escenaID, instanciaID uuid.UUID, currentSesionID *uuid.UUID) (*domain.EscenaInstancia, error) {
	snap, err := s.escenas.FindLatestSessionSnapshot(ctx, escenaID, instanciaID, currentSesionID)
	if err != nil {
		return nil, err
	}
	if err := s.escenas.UpdateInstanciaTransform(ctx, escenaID, instanciaID,
		&snap.PosX, &snap.PosY, &snap.PosZ, &snap.Escala,
		&snap.RotX, &snap.RotY, &snap.RotZ); err != nil {
		return nil, err
	}
	inst, err := s.escenas.FindInstancia(ctx, escenaID, instanciaID)
	if err != nil {
		return nil, err
	}
	if err := s.recordSessionSnapshot(ctx, actor, escenaID, currentSesionID, "restore_session", inst); err != nil {
		return nil, err
	}
	return inst, nil
}

// RemoveInstancia borra una instancia de la escena.
func (s *EscenaService) RemoveInstancia(ctx context.Context, actor, escenaID, instanciaID uuid.UUID) error {
	return s.escenas.DeleteInstancia(ctx, escenaID, instanciaID, actor)
}

func (s *EscenaService) recordSessionSnapshot(ctx context.Context, actor, escenaID uuid.UUID, sesionID *uuid.UUID, eventType string, inst *domain.EscenaInstancia) error {
	if sesionID == nil || inst == nil {
		return nil
	}
	return s.escenas.UpsertLabSesionInstancia(ctx, escenaID, *sesionID, actor, eventType, domain.LabSesionInstancia{
		InstanciaID: inst.ID,
		PosX:        inst.PosX,
		PosY:        inst.PosY,
		PosZ:        inst.PosZ,
		Escala:      inst.Escala,
		RotX:        inst.RotX,
		RotY:        inst.RotY,
		RotZ:        inst.RotZ,
	})
}

func isFinite(vals ...float64) bool {
	for _, v := range vals {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			return false
		}
	}
	return true
}

func allFinitePtrs(vals ...*float64) bool {
	for _, v := range vals {
		if v != nil && !isFinite(*v) {
			return false
		}
	}
	return true
}

func validHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, r := range s[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

func defaultSceneLight() domain.EscenaLight {
	return domain.EscenaLight{
		Activa:     false,
		Intensidad: 12,
		Color:      "#fff4d6",
		PosX:       4,
		PosY:       6,
		PosZ:       4,
		TargetX:    0,
		TargetY:    0,
		TargetZ:    0,
		Angulo:     0.55,
		Penumbra:   0.35,
		Distancia:  30,
		AutoTarget: false,
	}
}
