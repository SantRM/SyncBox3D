package service

import (
	"context"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/repository"
)

// NodoService implementa los casos de uso del árbol jerárquico.
type NodoService struct {
	repo *repository.NodoRepo
}

// NewNodoService construye el servicio.
func NewNodoService(r *repository.NodoRepo) *NodoService { return &NodoService{repo: r} }

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify normaliza un texto al formato aceptado por la columna slug.
func Slugify(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	out, _, err := transform.String(t, s)
	if err != nil {
		out = s
	}
	out = strings.ToLower(out)
	out = slugRe.ReplaceAllString(out, "_")
	out = strings.Trim(out, "_")
	return out
}

// NodoCreateInput entrada de alta.
type NodoCreateInput struct {
	Tipo     domain.NodoTipo `json:"tipo"`
	ParentID *uuid.UUID      `json:"parent_id,omitempty"`
	Nombre   string          `json:"nombre"`
	Slug     string          `json:"slug,omitempty"`
	Orden    int             `json:"orden,omitempty"`
}

// Create crea un nodo nuevo aplicando reglas de tipo.
func (s *NodoService) Create(ctx context.Context, actor uuid.UUID, in NodoCreateInput) (*domain.Nodo, error) {
	if !in.Tipo.Valid() || strings.TrimSpace(in.Nombre) == "" {
		return nil, domain.ErrInvalidInput
	}
	if in.Tipo == domain.NodoEquipo {
		return nil, domain.ErrNodoTipoInvalido
	}
	slug := strings.TrimSpace(in.Slug)
	if slug == "" {
		slug = Slugify(in.Nombre)
	} else {
		slug = Slugify(slug)
	}
	if slug == "" {
		return nil, domain.ErrInvalidInput
	}

	// Validación previa de tipo (la BD también la verifica vía trigger).
	if in.ParentID == nil && in.Tipo != domain.NodoUbicacion {
		return nil, domain.ErrNodoTipoInvalido
	}
	if in.ParentID != nil {
		parent, err := s.repo.FindByID(ctx, *in.ParentID)
		if err != nil {
			return nil, err
		}
		if parent.Tipo != domain.NodoUbicacion {
			return nil, domain.ErrNodoTipoInvalido
		}
		if in.Tipo == domain.NodoUbicacion && parent.Tipo != domain.NodoUbicacion {
			return nil, domain.ErrNodoTipoInvalido
		}
		if in.Tipo == domain.NodoLaboratorio && parent.Tipo != domain.NodoUbicacion {
			return nil, domain.ErrNodoTipoInvalido
		}
	}

	n := &domain.Nodo{
		Tipo: in.Tipo, ParentID: in.ParentID, Nombre: strings.TrimSpace(in.Nombre),
		Slug: slug, Orden: in.Orden,
	}
	if err := s.repo.Create(ctx, n, actor); err != nil {
		return nil, err
	}
	return n, nil
}

// NodoUpdateInput parche.
type NodoUpdateInput struct {
	Nombre *string `json:"nombre,omitempty"`
	Slug   *string `json:"slug,omitempty"`
	Orden  *int    `json:"orden,omitempty"`
}

// Update aplica un parche.
func (s *NodoService) Update(ctx context.Context, actor, id uuid.UUID, in NodoUpdateInput) (*domain.Nodo, error) {
	if _, err := s.repo.FindByID(ctx, id); err != nil {
		return nil, err
	}
	if in.Slug != nil {
		sl := Slugify(*in.Slug)
		if sl == "" {
			return nil, domain.ErrInvalidInput
		}
		in.Slug = &sl
	}
	if err := s.repo.Update(ctx, id, in.Nombre, in.Slug, in.Orden, actor); err != nil {
		return nil, err
	}
	return s.repo.FindByID(ctx, id)
}

// Move mueve un nodo a un nuevo padre con validación anti-ciclo y de tipos.
func (s *NodoService) Move(ctx context.Context, actor, id uuid.UUID, newParent *uuid.UUID) error {
	node, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if newParent == nil {
		if node.Tipo != domain.NodoUbicacion {
			return domain.ErrNodoTipoInvalido
		}
	} else {
		if *newParent == id {
			return domain.ErrNodoCiclo
		}
		parent, err := s.repo.FindByID(ctx, *newParent)
		if err != nil {
			return err
		}
		// Verificar que parent no sea descendiente del nodo (anti-ciclo).
		if strings.HasPrefix(parent.Path, node.Path+".") || parent.Path == node.Path {
			return domain.ErrNodoCiclo
		}
		// Reglas de tipo.
		switch node.Tipo {
		case domain.NodoUbicacion:
			if parent.Tipo != domain.NodoUbicacion {
				return domain.ErrNodoTipoInvalido
			}
		case domain.NodoLaboratorio:
			if parent.Tipo != domain.NodoUbicacion {
				return domain.ErrNodoTipoInvalido
			}
		case domain.NodoEquipo:
			if parent.Tipo != domain.NodoUbicacion {
				return domain.ErrNodoTipoInvalido
			}
		}
	}
	return s.repo.Move(ctx, id, newParent, actor)
}

// DeleteOpts controla el modo de borrado.
type DeleteOpts struct {
	Confirm             string     // debe ser "entiendo" para hojas
	ReplacementParentID *uuid.UUID // mover hijos a este padre
	Promote             bool       // si true, los hijos suben al abuelo
}

// Delete borra un nodo respetando reglas de hijos.
func (s *NodoService) Delete(ctx context.Context, actor, id uuid.UUID, opts DeleteOpts) error {
	node, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}
	hasChildren, err := s.repo.HasChildren(ctx, id)
	if err != nil {
		return err
	}
	if !hasChildren {
		if strings.ToLower(strings.TrimSpace(opts.Confirm)) != "entiendo" {
			return domain.ErrConfirmRequerida
		}
		return s.repo.SoftDelete(ctx, id, actor)
	}
	// Tiene hijos: requiere reasignación.
	var newParent uuid.UUID
	switch {
	case opts.ReplacementParentID != nil:
		newParent = *opts.ReplacementParentID
		if newParent == id {
			return domain.ErrNodoCiclo
		}
	case opts.Promote:
		if node.ParentID == nil {
			return domain.ErrNodoTienHijos
		}
		newParent = *node.ParentID
	default:
		return domain.ErrNodoTienHijos
	}
	if err := s.repo.Reparent(ctx, id, newParent, actor); err != nil {
		return err
	}
	return s.repo.SoftDelete(ctx, id, actor)
}

// Get devuelve un nodo por id.
func (s *NodoService) Get(ctx context.Context, id uuid.UUID) (*domain.Nodo, error) {
	return s.repo.FindByID(ctx, id)
}

// Roots devuelve los nodos raíz.
func (s *NodoService) Roots(ctx context.Context) ([]domain.Nodo, error) {
	return s.repo.ListRoots(ctx)
}

// Children devuelve los hijos directos.
func (s *NodoService) Children(ctx context.Context, parentID uuid.UUID) ([]domain.Nodo, error) {
	return s.repo.ListChildren(ctx, parentID)
}

// Subtree devuelve el subárbol completo.
func (s *NodoService) Subtree(ctx context.Context, id uuid.UUID) ([]domain.Nodo, error) {
	return s.repo.Subtree(ctx, id)
}

// Ancestors devuelve los ancestros.
func (s *NodoService) Ancestors(ctx context.Context, id uuid.UUID) ([]domain.Nodo, error) {
	return s.repo.Ancestors(ctx, id)
}
