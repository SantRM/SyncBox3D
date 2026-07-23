package service

import (
	"context"

	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/repository"
)

// CatalogService expone categorías y estados (catálogos de soporte).
type CatalogService struct {
	categorias *repository.CategoriaRepo
	estados    *repository.EstadoRepo
	audit      *repository.HistorialRepo
}

// NewCatalogService construye el servicio.
func NewCatalogService(c *repository.CategoriaRepo, e *repository.EstadoRepo, a *repository.HistorialRepo) *CatalogService {
	return &CatalogService{categorias: c, estados: e, audit: a}
}

// ListCategorias lista categorías.
func (s *CatalogService) ListCategorias(ctx context.Context, soloActivas bool) ([]domain.Categoria, error) {
	return s.categorias.List(ctx, soloActivas)
}

// CreateCategoria da de alta una categoría.
func (s *CatalogService) CreateCategoria(ctx context.Context, actor uuid.UUID, c *domain.Categoria) error {
	if c.Nombre == "" {
		return domain.ErrInvalidInput
	}
	c.Activo = true
	if err := s.categorias.Create(ctx, c); err != nil {
		return err
	}
	_ = s.audit.Record(ctx, repository.EntCategoria, c.ID, actor, []repository.Mutacion{
		{Campo: "alta", Anterior: "", Nuevo: c.Nombre},
	})
	return nil
}

// UpdateCategoria parche.
func (s *CatalogService) UpdateCategoria(ctx context.Context, actor, id uuid.UUID, nombre, descripcion *string, activo *bool) error {
	before, err := s.categorias.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.categorias.Update(ctx, id, nombre, descripcion, activo); err != nil {
		return err
	}
	var muts []repository.Mutacion
	if nombre != nil && *nombre != before.Nombre {
		muts = append(muts, repository.Mutacion{Campo: "nombre", Anterior: before.Nombre, Nuevo: *nombre})
	}
	if descripcion != nil && *descripcion != before.Descripcion {
		muts = append(muts, repository.Mutacion{Campo: "descripcion", Anterior: before.Descripcion, Nuevo: *descripcion})
	}
	if activo != nil && *activo != before.Activo {
		muts = append(muts, repository.Mutacion{Campo: "activo", Anterior: boolStr(before.Activo), Nuevo: boolStr(*activo)})
	}
	_ = s.audit.Record(ctx, repository.EntCategoria, id, actor, muts)
	return nil
}

// ListEstados devuelve los estados activos.
func (s *CatalogService) ListEstados(ctx context.Context) ([]domain.EstadoOperativo, error) {
	return s.estados.List(ctx)
}
