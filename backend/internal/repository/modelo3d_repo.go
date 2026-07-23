package repository

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gitlab.com/syncbox/backend/internal/domain"
)

// Modelo3DRepo agrupa accesos sobre la tabla `modelo_3d`.
type Modelo3DRepo struct{ p *Pool }

// NewModelo3DRepo construye el repositorio.
func NewModelo3DRepo(p *Pool) *Modelo3DRepo { return &Modelo3DRepo{p: p} }

const modelo3dCols = `id, nombre, COALESCE(descripcion,''), mime, tamano_bytes, sha256,
	storage_uri, COALESCE(preview_uri,''), activo, created_at, updated_at, created_by, updated_by`

func scanModelo3D(row pgx.Row) (*domain.Modelo3D, error) {
	var m domain.Modelo3D
	if err := row.Scan(&m.ID, &m.Nombre, &m.Descripcion, &m.Mime, &m.TamanoBytes, &m.SHA256,
		&m.StorageURI, &m.PreviewURI, &m.Activo,
		&m.CreatedAt, &m.UpdatedAt, &m.CreatedBy, &m.UpdatedBy); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &m, nil
}

// FindByID devuelve un modelo por id.
func (r *Modelo3DRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Modelo3D, error) {
	return scanModelo3D(r.p.QueryRow(ctx,
		`SELECT `+modelo3dCols+` FROM modelo_3d WHERE id = $1`, id))
}

// FindBySHA256 busca un modelo existente con el mismo hash (dedup).
func (r *Modelo3DRepo) FindBySHA256(ctx context.Context, sha string) (*domain.Modelo3D, error) {
	return scanModelo3D(r.p.QueryRow(ctx,
		`SELECT `+modelo3dCols+` FROM modelo_3d WHERE sha256 = $1`, sha))
}

// List devuelve todos los modelos activos.
func (r *Modelo3DRepo) List(ctx context.Context, search string, limit int) ([]domain.Modelo3D, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	rows, err := r.p.Query(ctx, `
		SELECT `+modelo3dCols+`
		FROM modelo_3d
		WHERE activo = true
		  AND ($1 = '' OR nombre ILIKE '%'||$1||'%')
		ORDER BY updated_at DESC
		LIMIT $2`, search, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]domain.Modelo3D, 0)
	for rows.Next() {
		var m domain.Modelo3D
		if err := rows.Scan(&m.ID, &m.Nombre, &m.Descripcion, &m.Mime, &m.TamanoBytes, &m.SHA256,
			&m.StorageURI, &m.PreviewURI, &m.Activo,
			&m.CreatedAt, &m.UpdatedAt, &m.CreatedBy, &m.UpdatedBy); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// Create inserta un modelo. Si sha256 ya existe devuelve ErrConflict.
func (r *Modelo3DRepo) Create(ctx context.Context, m *domain.Modelo3D, actor uuid.UUID) error {
	row := r.p.QueryRow(ctx, `
		INSERT INTO modelo_3d (nombre, descripcion, mime, tamano_bytes, sha256,
		                       storage_uri, preview_uri, created_by, updated_by)
		VALUES ($1, NULLIF($2,''), $3, $4, $5, $6, NULLIF($7,''), $8, $8)
		RETURNING `+modelo3dCols,
		m.Nombre, m.Descripcion, m.Mime, m.TamanoBytes, m.SHA256,
		m.StorageURI, m.PreviewURI, actor)
	res, err := scanModelo3D(row)
	if err != nil {
		return MapPgError(err)
	}
	*m = *res
	return nil
}

// Update permite renombrar/redescribir.
func (r *Modelo3DRepo) Update(ctx context.Context, id uuid.UUID, nombre, descripcion *string, actor uuid.UUID) error {
	tag, err := r.p.Exec(ctx, `
		UPDATE modelo_3d SET
			nombre      = COALESCE($2, nombre),
			descripcion = COALESCE($3, descripcion),
			updated_by  = $4
		WHERE id = $1`, id, nombre, descripcion, actor)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// Delete borra el modelo si no está referenciado por ningún equipo vivo.
func (r *Modelo3DRepo) Delete(ctx context.Context, id uuid.UUID) error {
	var inUse int
	if err := r.p.QueryRow(ctx,
		`SELECT COUNT(*) FROM equipo WHERE modelo_3d_id = $1 AND deleted_at IS NULL`,
		id).Scan(&inUse); err != nil {
		return err
	}
	if inUse > 0 {
		return domain.ErrModeloEnUso
	}
	tag, err := r.p.Exec(ctx, `DELETE FROM modelo_3d WHERE id = $1`, id)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
