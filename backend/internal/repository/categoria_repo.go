package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gitlab.com/syncbox/backend/internal/domain"
)

// CategoriaRepo gestiona la tabla `categoria`.
type CategoriaRepo struct{ p *Pool }

// NewCategoriaRepo construye el repositorio.
func NewCategoriaRepo(p *Pool) *CategoriaRepo { return &CategoriaRepo{p: p} }

// List devuelve todas las categorías ordenadas por nombre.
func (r *CategoriaRepo) List(ctx context.Context, soloActivas bool) ([]domain.Categoria, error) {
	q := `SELECT id, nombre, COALESCE(descripcion,''), activo, created_at, updated_at FROM categoria`
	if soloActivas {
		q += ` WHERE activo = TRUE`
	}
	q += ` ORDER BY nombre`
	rows, err := r.p.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Categoria
	for rows.Next() {
		var c domain.Categoria
		if err := rows.Scan(&c.ID, &c.Nombre, &c.Descripcion, &c.Activo, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// FindByID recupera una categoría.
func (r *CategoriaRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Categoria, error) {
	var c domain.Categoria
	err := r.p.QueryRow(ctx,
		`SELECT id, nombre, COALESCE(descripcion,''), activo, created_at, updated_at FROM categoria WHERE id = $1`, id,
	).Scan(&c.ID, &c.Nombre, &c.Descripcion, &c.Activo, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

// Create inserta una categoría.
func (r *CategoriaRepo) Create(ctx context.Context, c *domain.Categoria) error {
	return MapPgError(r.p.QueryRow(ctx, `
		INSERT INTO categoria (nombre, descripcion, activo)
		VALUES ($1, NULLIF($2,''), TRUE)
		RETURNING id, created_at, updated_at`,
		c.Nombre, c.Descripcion,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt))
}

// Update modifica una categoría.
func (r *CategoriaRepo) Update(ctx context.Context, id uuid.UUID, nombre, descripcion *string, activo *bool) error {
	tag, err := r.p.Exec(ctx, `
		UPDATE categoria SET
			nombre      = COALESCE($2, nombre),
			descripcion = COALESCE($3, descripcion),
			activo      = COALESCE($4, activo)
		WHERE id = $1`, id, nombre, descripcion, activo)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
