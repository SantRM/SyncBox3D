package service

import (
	"context"
	"strconv"

	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/repository"
)

func formatFloat(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }
func formatInt(v int) string       { return strconv.Itoa(v) }

// EquipoService implementa los casos de uso de gestión de equipos.
type EquipoService struct {
	equipos    *repository.EquipoRepo
	categorias *repository.CategoriaRepo
	estados    *repository.EstadoRepo
	audit      *repository.HistorialRepo
	fichas     *repository.FichaRepo
	nodos      *repository.NodoRepo
	modelos3d  *repository.Modelo3DRepo
}

// NewEquipoService construye el servicio.
func NewEquipoService(e *repository.EquipoRepo, c *repository.CategoriaRepo, es *repository.EstadoRepo, a *repository.HistorialRepo, f *repository.FichaRepo, nodos *repository.NodoRepo, modelos3d *repository.Modelo3DRepo) *EquipoService {
	return &EquipoService{
		equipos: e, categorias: c, estados: es, audit: a, fichas: f, nodos: nodos, modelos3d: modelos3d,
	}
}

// ListInput parametriza la consulta.
type ListInput struct {
	CategoriaID *uuid.UUID
	EstadoID    *uuid.UUID
	NodoID      *uuid.UUID
	Search      string
	Limit       int
	Offset      int
}

// List devuelve equipos vigentes.
func (s *EquipoService) List(ctx context.Context, in ListInput) ([]domain.Equipo, error) {
	return s.equipos.List(ctx, in.CategoriaID, in.EstadoID, in.NodoID, in.Search, in.Limit, in.Offset)
}

// Get devuelve un equipo por id.
func (s *EquipoService) Get(ctx context.Context, id uuid.UUID) (*domain.Equipo, error) {
	return s.equipos.FindByID(ctx, id)
}

// EquipoCreateInput entrada de alta.
type EquipoCreateInput struct {
	CategoriaID  uuid.UUID  `json:"categoria_id"`
	EstadoID     uuid.UUID  `json:"estado_id"`
	Nombre       string     `json:"nombre"`
	Fabricante   string     `json:"fabricante,omitempty"`
	Modelo       string     `json:"modelo,omitempty"`
	Serial       string     `json:"serial,omitempty"`
	Ubicacion    string     `json:"ubicacion,omitempty"`
	ParentNodoID *uuid.UUID `json:"parent_nodo_id,omitempty"`
	NodoID       *uuid.UUID `json:"nodo_id,omitempty"`
	Modelo3DID   *uuid.UUID `json:"modelo_3d_id,omitempty"`
}

// Create da de alta un equipo.
func (s *EquipoService) Create(ctx context.Context, actor uuid.UUID, in EquipoCreateInput) (*domain.Equipo, error) {
	if in.Nombre == "" {
		return nil, domain.ErrInvalidInput
	}
	if _, err := s.categorias.FindByID(ctx, in.CategoriaID); err != nil {
		return nil, domain.ErrInvalidInput
	}
	if ok, _ := s.estados.Exists(ctx, in.EstadoID); !ok {
		return nil, domain.ErrInvalidInput
	}
	if err := s.validateModelo3D(ctx, in.Modelo3DID); err != nil {
		return nil, err
	}
	if in.ParentNodoID == nil && in.NodoID == nil {
		return nil, domain.ErrInvalidInput
	}
	if in.ParentNodoID != nil && in.NodoID != nil {
		return nil, domain.ErrInvalidInput
	}
	if in.ParentNodoID != nil {
		parent, err := s.nodos.FindByID(ctx, *in.ParentNodoID)
		if err != nil {
			return nil, domain.ErrInvalidInput
		}
		if parent.Tipo != domain.NodoUbicacion {
			return nil, domain.ErrNodoTipoInvalido
		}
		e := &domain.Equipo{
			CategoriaID: in.CategoriaID, EstadoID: in.EstadoID,
			Nombre: in.Nombre, Fabricante: in.Fabricante, Modelo: in.Modelo,
			Serial: in.Serial, Ubicacion: in.Ubicacion,
			Modelo3DID: in.Modelo3DID,
		}
		if err := s.equipos.CreateWithNodo(ctx, e, *in.ParentNodoID, Slugify(in.Nombre), 0, actor); err != nil {
			return nil, err
		}
		return e, nil
	}
	if in.NodoID != nil {
		node, err := s.nodos.FindByID(ctx, *in.NodoID)
		if err != nil {
			return nil, domain.ErrInvalidInput
		}
		if node.Tipo != domain.NodoEquipo {
			return nil, domain.ErrNodoTipoInvalido
		}
	}
	e := &domain.Equipo{
		CategoriaID: in.CategoriaID, EstadoID: in.EstadoID,
		Nombre: in.Nombre, Fabricante: in.Fabricante, Modelo: in.Modelo,
		Serial: in.Serial, Ubicacion: in.Ubicacion,
		NodoID: in.NodoID, Modelo3DID: in.Modelo3DID,
	}
	if err := s.equipos.Create(ctx, e, actor); err != nil {
		return nil, err
	}
	return e, nil
}

// EquipoUpdateInput parche de campos editables (no estado).
type EquipoUpdateInput struct {
	CategoriaID *uuid.UUID `json:"categoria_id,omitempty"`
	Nombre      *string    `json:"nombre,omitempty"`
	Fabricante  *string    `json:"fabricante,omitempty"`
	Modelo      *string    `json:"modelo,omitempty"`
	Serial      *string    `json:"serial,omitempty"`
	Ubicacion   *string    `json:"ubicacion,omitempty"`
	NodoID      *uuid.UUID `json:"nodo_id,omitempty"`
}

// Update aplica un parche.
func (s *EquipoService) Update(ctx context.Context, actor, id uuid.UUID, in EquipoUpdateInput) (*domain.Equipo, error) {
	before, err := s.equipos.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if in.CategoriaID != nil {
		if _, err := s.categorias.FindByID(ctx, *in.CategoriaID); err != nil {
			return nil, domain.ErrInvalidInput
		}
	}
	if in.NodoID != nil {
		if before.NodoID == nil || *in.NodoID != *before.NodoID {
			return nil, domain.ErrInvalidInput
		}
		node, err := s.nodos.FindByID(ctx, *in.NodoID)
		if err != nil {
			return nil, domain.ErrInvalidInput
		}
		if node.Tipo != domain.NodoEquipo {
			return nil, domain.ErrNodoTipoInvalido
		}
	}
	if err := s.equipos.Update(ctx, id, in.Nombre, in.Fabricante, in.Modelo, in.Serial, in.Ubicacion, in.CategoriaID, in.NodoID, actor); err != nil {
		return nil, err
	}
	after, err := s.equipos.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Record(ctx, repository.EntEquipo, id, actor, buildEquipoMutaciones(before, after))
	return after, nil
}

// SetModelo3D asigna o limpia el modelo reutilizable asociado al equipo.
func (s *EquipoService) SetModelo3D(ctx context.Context, actor, id uuid.UUID, modelo3DID *uuid.UUID) (*domain.Equipo, error) {
	before, err := s.equipos.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.validateModelo3D(ctx, modelo3DID); err != nil {
		return nil, err
	}
	if err := s.equipos.SetModelo3D(ctx, id, modelo3DID, actor); err != nil {
		return nil, err
	}
	after, err := s.equipos.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Record(ctx, repository.EntEquipo, id, actor, buildEquipoMutaciones(before, after))
	return after, nil
}

func (s *EquipoService) validateModelo3D(ctx context.Context, modelo3DID *uuid.UUID) error {
	if modelo3DID == nil {
		return nil
	}
	if _, err := s.modelos3d.FindByID(ctx, *modelo3DID); err != nil {
		if err == domain.ErrNotFound {
			return domain.ErrInvalidInput
		}
		return err
	}
	return nil
}

// Delete soft-delete.
func (s *EquipoService) Delete(ctx context.Context, actor, id uuid.UUID) error {
	before, err := s.equipos.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.equipos.SoftDelete(ctx, id, actor); err != nil {
		return err
	}
	if before.NodoID != nil {
		_ = s.nodos.SoftDelete(ctx, *before.NodoID, actor)
	}
	_ = s.audit.Record(ctx, repository.EntEquipo, id, actor, []repository.Mutacion{
		{Campo: "deleted_at", Anterior: "", Nuevo: "now"},
	})
	return nil
}

// ChangeState transiciona el estado de un equipo (atómico con historial).
func (s *EquipoService) ChangeState(ctx context.Context, actor, equipoID, nuevoEstado uuid.UUID, motivo string) error {
	if ok, _ := s.estados.Exists(ctx, nuevoEstado); !ok {
		return domain.ErrInvalidInput
	}
	return s.equipos.ChangeState(ctx, equipoID, nuevoEstado, actor, motivo)
}

func buildEquipoMutaciones(before, after *domain.Equipo) []repository.Mutacion {
	var muts []repository.Mutacion
	if before.Nombre != after.Nombre {
		muts = append(muts, repository.Mutacion{Campo: "nombre", Anterior: before.Nombre, Nuevo: after.Nombre})
	}
	if before.Fabricante != after.Fabricante {
		muts = append(muts, repository.Mutacion{Campo: "fabricante", Anterior: before.Fabricante, Nuevo: after.Fabricante})
	}
	if before.Modelo != after.Modelo {
		muts = append(muts, repository.Mutacion{Campo: "modelo", Anterior: before.Modelo, Nuevo: after.Modelo})
	}
	if before.Serial != after.Serial {
		muts = append(muts, repository.Mutacion{Campo: "serial", Anterior: before.Serial, Nuevo: after.Serial})
	}
	if before.Ubicacion != after.Ubicacion {
		muts = append(muts, repository.Mutacion{Campo: "ubicacion", Anterior: before.Ubicacion, Nuevo: after.Ubicacion})
	}
	if before.CategoriaID != after.CategoriaID {
		muts = append(muts, repository.Mutacion{Campo: "categoria_id", Anterior: before.CategoriaID.String(), Nuevo: after.CategoriaID.String()})
	}
	uuidStr := func(u *uuid.UUID) string {
		if u == nil {
			return ""
		}
		return u.String()
	}
	if uuidStr(before.NodoID) != uuidStr(after.NodoID) {
		muts = append(muts, repository.Mutacion{Campo: "nodo_id", Anterior: uuidStr(before.NodoID), Nuevo: uuidStr(after.NodoID)})
	}
	if uuidStr(before.Modelo3DID) != uuidStr(after.Modelo3DID) {
		muts = append(muts, repository.Mutacion{Campo: "modelo_3d_id", Anterior: uuidStr(before.Modelo3DID), Nuevo: uuidStr(after.Modelo3DID)})
	}
	return muts
}

// ListEstadoHistorial expone el historial de cambios de estado.
func (s *EquipoService) ListEstadoHistorial(ctx context.Context, equipoID uuid.UUID) ([]repository.EstadoHistorialEntry, error) {
	if _, err := s.equipos.FindByID(ctx, equipoID); err != nil {
		return nil, err
	}
	return s.equipos.ListEstadoHistorial(ctx, equipoID)
}

// ListCambios expone el historial de cambios a nivel de campo.
func (s *EquipoService) ListCambios(ctx context.Context, equipoID uuid.UUID) ([]repository.CambioEntry, error) {
	if _, err := s.equipos.FindByID(ctx, equipoID); err != nil {
		return nil, err
	}
	return s.equipos.ListCambios(ctx, equipoID)
}

// GetFicha devuelve la ficha técnica del equipo o nil si no existe.
func (s *EquipoService) GetFicha(ctx context.Context, equipoID uuid.UUID) (*repository.FichaTecnica, error) {
	if _, err := s.equipos.FindByID(ctx, equipoID); err != nil {
		return nil, err
	}
	f, err := s.fichas.Get(ctx, equipoID)
	if err != nil {
		if err == domain.ErrNotFound {
			return nil, nil
		}
		return nil, err
	}
	return f, nil
}

// UpsertFicha crea o actualiza la ficha técnica.
func (s *EquipoService) UpsertFicha(ctx context.Context, actor, equipoID uuid.UUID, f *repository.FichaTecnica) (*repository.FichaTecnica, error) {
	if _, err := s.equipos.FindByID(ctx, equipoID); err != nil {
		return nil, err
	}
	before, _ := s.fichas.Get(ctx, equipoID)
	f.EquipoID = equipoID
	if err := s.fichas.Upsert(ctx, f); err != nil {
		return nil, err
	}
	muts := buildFichaMutaciones(before, f)
	if len(muts) > 0 {
		_ = s.audit.Record(ctx, repository.EntFichaTecnica, equipoID, actor, muts)
	}
	return f, nil
}

func buildFichaMutaciones(before, after *repository.FichaTecnica) []repository.Mutacion {
	var muts []repository.Mutacion
	prev := func(f *float64) string {
		if f == nil {
			return ""
		}
		return formatFloat(*f)
	}
	prevInt := func(i *int) string {
		if i == nil {
			return ""
		}
		return formatInt(*i)
	}
	var b repository.FichaTecnica
	if before != nil {
		b = *before
	}
	if prev(b.Peso) != prev(after.Peso) {
		muts = append(muts, repository.Mutacion{Campo: "peso", Anterior: prev(b.Peso), Nuevo: prev(after.Peso)})
	}
	if prev(b.Potencia) != prev(after.Potencia) {
		muts = append(muts, repository.Mutacion{Campo: "potencia", Anterior: prev(b.Potencia), Nuevo: prev(after.Potencia)})
	}
	if b.Dimensiones != after.Dimensiones {
		muts = append(muts, repository.Mutacion{Campo: "dimensiones", Anterior: b.Dimensiones, Nuevo: after.Dimensiones})
	}
	if prevInt(b.Anio) != prevInt(after.Anio) {
		muts = append(muts, repository.Mutacion{Campo: "anio", Anterior: prevInt(b.Anio), Nuevo: prevInt(after.Anio)})
	}
	if b.Observaciones != after.Observaciones {
		muts = append(muts, repository.Mutacion{Campo: "observaciones", Anterior: b.Observaciones, Nuevo: after.Observaciones})
	}
	return muts
}
