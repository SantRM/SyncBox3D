package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gitlab.com/syncbox/backend/internal/domain"
)

// NodoRepo agrupa accesos sobre la tabla `nodo` y su árbol ltree.
type NodoRepo struct{ p *Pool }

// NewNodoRepo construye el repositorio.
func NewNodoRepo(p *Pool) *NodoRepo { return &NodoRepo{p: p} }

const nodoCols = `id, tipo, parent_id, nombre, slug, orden,
	path::text, depth, activo, deleted_at, created_at, updated_at, created_by, updated_by`

func scanNodo(row pgx.Row) (*domain.Nodo, error) {
	var n domain.Nodo
	var tipo string
	if err := row.Scan(&n.ID, &tipo, &n.ParentID, &n.Nombre, &n.Slug, &n.Orden,
		&n.Path, &n.Depth, &n.Activo, &n.DeletedAt,
		&n.CreatedAt, &n.UpdatedAt, &n.CreatedBy, &n.UpdatedBy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	n.Tipo = domain.NodoTipo(tipo)
	return &n, nil
}

func scanNodoRows(rows pgx.Rows) ([]domain.Nodo, error) {
	defer rows.Close()
	out := make([]domain.Nodo, 0)
	for rows.Next() {
		var n domain.Nodo
		var tipo string
		if err := rows.Scan(&n.ID, &tipo, &n.ParentID, &n.Nombre, &n.Slug, &n.Orden,
			&n.Path, &n.Depth, &n.Activo, &n.DeletedAt,
			&n.CreatedAt, &n.UpdatedAt, &n.CreatedBy, &n.UpdatedBy); err != nil {
			return nil, err
		}
		n.Tipo = domain.NodoTipo(tipo)
		out = append(out, n)
	}
	return out, rows.Err()
}

// FindByID devuelve un nodo no eliminado.
func (r *NodoRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Nodo, error) {
	row := r.p.QueryRow(ctx,
		`SELECT `+nodoCols+` FROM nodo WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanNodo(row)
}

// ListRoots devuelve los nodos raíz (parent_id IS NULL).
func (r *NodoRepo) ListRoots(ctx context.Context) ([]domain.Nodo, error) {
	rows, err := r.p.Query(ctx,
		`SELECT `+nodoCols+` FROM nodo
		 WHERE parent_id IS NULL AND deleted_at IS NULL
		 ORDER BY orden, nombre`)
	if err != nil {
		return nil, err
	}
	return scanNodoRows(rows)
}

// ListChildren devuelve los hijos directos de un nodo.
func (r *NodoRepo) ListChildren(ctx context.Context, parentID uuid.UUID) ([]domain.Nodo, error) {
	rows, err := r.p.Query(ctx,
		`SELECT `+nodoCols+` FROM nodo
		 WHERE parent_id = $1 AND deleted_at IS NULL
		 ORDER BY orden, nombre`, parentID)
	if err != nil {
		return nil, err
	}
	return scanNodoRows(rows)
}

// Subtree devuelve todos los descendientes (incluido el propio) de un nodo.
func (r *NodoRepo) Subtree(ctx context.Context, id uuid.UUID) ([]domain.Nodo, error) {
	rows, err := r.p.Query(ctx, `
		SELECT `+nodoCols+` FROM nodo
		WHERE path <@ (SELECT path FROM nodo WHERE id = $1)
		  AND deleted_at IS NULL
		ORDER BY path`, id)
	if err != nil {
		return nil, err
	}
	return scanNodoRows(rows)
}

// Ancestors devuelve los ancestros de un nodo en orden raíz→padre.
func (r *NodoRepo) Ancestors(ctx context.Context, id uuid.UUID) ([]domain.Nodo, error) {
	rows, err := r.p.Query(ctx, `
		SELECT `+nodoCols+` FROM nodo
		WHERE path @> (SELECT path FROM nodo WHERE id = $1)
		  AND deleted_at IS NULL AND id <> $1
		ORDER BY path`, id)
	if err != nil {
		return nil, err
	}
	return scanNodoRows(rows)
}

// HasChildren indica si el nodo tiene al menos un hijo no eliminado.
func (r *NodoRepo) HasChildren(ctx context.Context, id uuid.UUID) (bool, error) {
	var exists bool
	err := r.p.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM nodo WHERE parent_id = $1 AND deleted_at IS NULL)`,
		id).Scan(&exists)
	return exists, err
}

// Create inserta un nodo nuevo. El path se calcula por trigger.
func (r *NodoRepo) Create(ctx context.Context, n *domain.Nodo, actor uuid.UUID) error {
	row := r.p.QueryRow(ctx, `
		INSERT INTO nodo (tipo, parent_id, nombre, slug, orden, path, created_by, updated_by)
		VALUES ($1, $2, $3, $4, COALESCE($5,0), ''::ltree, $6, $6)
		RETURNING `+nodoCols,
		string(n.Tipo), n.ParentID, n.Nombre, n.Slug, n.Orden, actor)
	res, err := scanNodo(row)
	if err != nil {
		return MapPgError(err)
	}
	*n = *res
	return nil
}

// Update modifica nombre/slug/orden. Si slug cambia, el trigger recalcula path.
func (r *NodoRepo) Update(ctx context.Context, id uuid.UUID, nombre, slug *string, orden *int, actor uuid.UUID) error {
	tag, err := r.p.Exec(ctx, `
		UPDATE nodo SET
			nombre     = COALESCE($2, nombre),
			slug       = COALESCE($3, slug),
			orden      = COALESCE($4, orden),
			updated_by = $5
		WHERE id = $1 AND deleted_at IS NULL`,
		id, nombre, slug, orden, actor)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Move cambia el parent_id; el trigger recalcula path y propaga descendientes.
func (r *NodoRepo) Move(ctx context.Context, id uuid.UUID, newParent *uuid.UUID, actor uuid.UUID) error {
	tag, err := r.p.Exec(ctx,
		`UPDATE nodo SET parent_id = $2, updated_by = $3 WHERE id = $1 AND deleted_at IS NULL`,
		id, newParent, actor)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Reparent reasigna en lote los hijos directos de oldParent a newParent.
func (r *NodoRepo) Reparent(ctx context.Context, oldParent, newParent uuid.UUID, actor uuid.UUID) error {
	_, err := r.p.Exec(ctx,
		`UPDATE nodo SET parent_id = $2, updated_by = $3
		 WHERE parent_id = $1 AND deleted_at IS NULL`,
		oldParent, newParent, actor)
	return MapPgError(err)
}

// SoftDelete marca el nodo como borrado.
func (r *NodoRepo) SoftDelete(ctx context.Context, id uuid.UUID, actor uuid.UUID) error {
	tag, err := r.p.Exec(ctx,
		`UPDATE nodo SET deleted_at = NOW(), activo = false, updated_by = $2
		 WHERE id = $1 AND deleted_at IS NULL`, id, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// CountEquiposEnSubtree cuenta equipos vivos cuyos nodos cuelgan del subárbol.
func (r *NodoRepo) CountEquiposEnSubtree(ctx context.Context, id uuid.UUID) (int, error) {
	var n int
	err := r.p.QueryRow(ctx, `
		SELECT COUNT(*) FROM equipo e
		JOIN nodo n ON n.id = e.nodo_id
		WHERE e.deleted_at IS NULL
		  AND n.path <@ (SELECT path FROM nodo WHERE id = $1)`, id).Scan(&n)
	return n, err
}
