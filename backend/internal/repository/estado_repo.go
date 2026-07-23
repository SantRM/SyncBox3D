package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gitlab.com/syncbox/backend/internal/domain"
)

// EstadoRepo gestiona `estado_operativo`.
type EstadoRepo struct{ p *Pool }

// NewEstadoRepo construye el repositorio.
func NewEstadoRepo(p *Pool) *EstadoRepo { return &EstadoRepo{p: p} }

// List devuelve los estados operativos ordenados por `orden`.
func (r *EstadoRepo) List(ctx context.Context) ([]domain.EstadoOperativo, error) {
	rows, err := r.p.Query(ctx, `
		SELECT id, nombre, color, orden, activo
		FROM estado_operativo WHERE activo = TRUE ORDER BY orden, nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.EstadoOperativo
	for rows.Next() {
		var e domain.EstadoOperativo
		if err := rows.Scan(&e.ID, &e.Nombre, &e.Color, &e.Orden, &e.Activo); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Exists verifica si un estado existe y está activo.
func (r *EstadoRepo) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	var ok bool
	err := r.p.QueryRow(ctx,
		`SELECT TRUE FROM estado_operativo WHERE id = $1 AND activo = TRUE`, id).Scan(&ok)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return ok, nil
}
