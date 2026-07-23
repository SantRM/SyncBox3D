package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gitlab.com/syncbox/backend/internal/domain"
)

// EquipoRepo agrupa accesos sobre `equipo` y su historial de estado.
type EquipoRepo struct{ p *Pool }

// NewEquipoRepo construye el repositorio.
func NewEquipoRepo(p *Pool) *EquipoRepo { return &EquipoRepo{p: p} }

const equipoCols = `id, nombre, COALESCE(fabricante,''), COALESCE(modelo,''), COALESCE(serial,''),
	COALESCE(ubicacion,''), nodo_id, modelo_3d_id,
	categoria_id, estado_id, estado_desde, activo, deleted_at,
	created_at, updated_at, updated_by`

func scanEquipo(row pgx.Row) (*domain.Equipo, error) {
	var e domain.Equipo
	err := row.Scan(&e.ID, &e.Nombre, &e.Fabricante, &e.Modelo, &e.Serial, &e.Ubicacion,
		&e.NodoID, &e.Modelo3DID,
		&e.CategoriaID, &e.EstadoID, &e.EstadoDesde, &e.Activo, &e.DeletedAt,
		&e.CreatedAt, &e.UpdatedAt, &e.UpdatedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &e, nil
}

// List filtra por categoría, estado y nodo (subárbol) opcionales y excluye soft-deleted.
func (r *EquipoRepo) List(ctx context.Context, categoriaID, estadoID, nodoID *uuid.UUID, search string, limit, offset int) ([]domain.Equipo, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.p.Query(ctx, `
		SELECT `+equipoCols+`
		FROM equipo e
		WHERE e.deleted_at IS NULL
		  AND ($1::uuid IS NULL OR e.categoria_id = $1)
		  AND ($2::uuid IS NULL OR e.estado_id    = $2)
		  AND ($3 = '' OR e.nombre ILIKE '%'||$3||'%' OR COALESCE(e.serial,'') ILIKE '%'||$3||'%')
		  AND ($4::uuid IS NULL OR EXISTS (
		       SELECT 1 FROM nodo n_root, nodo n_eq
		       WHERE n_root.id = $4 AND n_eq.id = e.nodo_id AND n_eq.path <@ n_root.path))
		ORDER BY e.updated_at DESC
		LIMIT $5 OFFSET $6`,
		categoriaID, estadoID, search, nodoID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Equipo
	for rows.Next() {
		e, err := scanEquipoRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *e)
	}
	return out, rows.Err()
}

func scanEquipoRow(rows pgx.Rows) (*domain.Equipo, error) {
	var e domain.Equipo
	err := rows.Scan(&e.ID, &e.Nombre, &e.Fabricante, &e.Modelo, &e.Serial, &e.Ubicacion,
		&e.NodoID, &e.Modelo3DID,
		&e.CategoriaID, &e.EstadoID, &e.EstadoDesde, &e.Activo, &e.DeletedAt,
		&e.CreatedAt, &e.UpdatedAt, &e.UpdatedBy)
	return &e, err
}

// FindByID recupera un equipo no eliminado.
func (r *EquipoRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Equipo, error) {
	row := r.p.QueryRow(ctx, `SELECT `+equipoCols+` FROM equipo WHERE id = $1 AND deleted_at IS NULL`, id)
	return scanEquipo(row)
}

// Create inserta un equipo nuevo y registra su alta en el historial de
// estados (sin estado anterior).
func (r *EquipoRepo) Create(ctx context.Context, e *domain.Equipo, actor uuid.UUID) error {
	return r.p.WithTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO equipo (nombre, fabricante, modelo, serial, ubicacion,
			                    nodo_id, modelo_3d_id,
			                    categoria_id, estado_id, estado_desde, activo, updated_by)
			VALUES ($1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), NULLIF($5,''),
			        $6, $7,
			        $8, $9, NOW(), TRUE, $10)
			RETURNING id, estado_desde, activo, created_at, updated_at`,
			e.Nombre, e.Fabricante, e.Modelo, e.Serial, e.Ubicacion,
			e.NodoID, e.Modelo3DID,
			e.CategoriaID, e.EstadoID, actor,
		).Scan(&e.ID, &e.EstadoDesde, &e.Activo, &e.CreatedAt, &e.UpdatedAt)
		if err != nil {
			return MapPgError(err)
		}
		e.UpdatedBy = &actor
		_, err = tx.Exec(ctx, `
			INSERT INTO equipo_estado_historial
			    (equipo_id, estado_anterior_id, estado_nuevo_id, usuario_id, motivo)
			VALUES ($1, NULL, $2, $3, 'Alta del equipo')`,
			e.ID, e.EstadoID, actor)
		return err
	})
}

// CreateWithNodo crea atomically el nodo EQUIPO bajo parentNodoID y el registro
// de equipo enlazado a ese nodo.
func (r *EquipoRepo) CreateWithNodo(ctx context.Context, e *domain.Equipo, parentNodoID uuid.UUID, nodeSlug string, nodeOrden int, actor uuid.UUID) error {
	return r.p.WithTx(ctx, func(tx pgx.Tx) error {
		var nodoID uuid.UUID
		if err := tx.QueryRow(ctx, `
			INSERT INTO nodo (tipo, parent_id, nombre, slug, orden, path, created_by, updated_by)
			VALUES ('EQUIPO', $1, $2, $3, COALESCE($4,0), ''::ltree, $5, $5)
			RETURNING id`,
			parentNodoID, e.Nombre, nodeSlug, nodeOrden, actor,
		).Scan(&nodoID); err != nil {
			return MapPgError(err)
		}
		e.NodoID = &nodoID

		err := tx.QueryRow(ctx, `
			INSERT INTO equipo (nombre, fabricante, modelo, serial, ubicacion,
			                    nodo_id, modelo_3d_id,
			                    categoria_id, estado_id, estado_desde, activo, updated_by)
			VALUES ($1, NULLIF($2,''), NULLIF($3,''), NULLIF($4,''), NULLIF($5,''),
			        $6, $7,
			        $8, $9, NOW(), TRUE, $10)
			RETURNING id, estado_desde, activo, created_at, updated_at`,
			e.Nombre, e.Fabricante, e.Modelo, e.Serial, e.Ubicacion,
			e.NodoID, e.Modelo3DID,
			e.CategoriaID, e.EstadoID, actor,
		).Scan(&e.ID, &e.EstadoDesde, &e.Activo, &e.CreatedAt, &e.UpdatedAt)
		if err != nil {
			return MapPgError(err)
		}
		e.UpdatedBy = &actor
		_, err = tx.Exec(ctx, `
			INSERT INTO equipo_estado_historial
			    (equipo_id, estado_anterior_id, estado_nuevo_id, usuario_id, motivo)
			VALUES ($1, NULL, $2, $3, 'Alta del equipo')`,
			e.ID, e.EstadoID, actor)
		return err
	})
}

// Update aplica un parche.
func (r *EquipoRepo) Update(ctx context.Context, id uuid.UUID, nombre, fabricante, modelo, serial, ubicacion *string, categoriaID, nodoID *uuid.UUID, actor uuid.UUID) error {
	tag, err := r.p.Exec(ctx, `
		UPDATE equipo SET
			nombre       = COALESCE($2, nombre),
			fabricante   = COALESCE($3, fabricante),
			modelo       = COALESCE($4, modelo),
			serial       = COALESCE($5, serial),
			ubicacion    = COALESCE($6, ubicacion),
			categoria_id = COALESCE($7, categoria_id),
			nodo_id      = COALESCE($8, nodo_id),
			updated_by   = $9
		WHERE id = $1 AND deleted_at IS NULL`,
		id, nombre, fabricante, modelo, serial, ubicacion, categoriaID, nodoID, actor)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SetModelo3D asigna exactamente el modelo reutilizable del equipo. Acepta nil
// para dejar el equipo sin modelo 3D.
func (r *EquipoRepo) SetModelo3D(ctx context.Context, id uuid.UUID, modelo3DID *uuid.UUID, actor uuid.UUID) error {
	tag, err := r.p.Exec(ctx, `
		UPDATE equipo SET
			modelo_3d_id = $2,
			updated_by   = $3
		WHERE id = $1 AND deleted_at IS NULL`,
		id, modelo3DID, actor)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SoftDelete marca deleted_at = NOW().
func (r *EquipoRepo) SoftDelete(ctx context.Context, id uuid.UUID, actor uuid.UUID) error {
	tag, err := r.p.Exec(ctx,
		`UPDATE equipo SET deleted_at = NOW(), updated_by = $2 WHERE id = $1 AND deleted_at IS NULL`,
		id, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ChangeState aplica un cambio de estado en una sola transacción: actualiza el
// equipo (estado_id, estado_desde) y persiste un registro en
// `equipo_estado_historial`.
func (r *EquipoRepo) ChangeState(ctx context.Context, equipoID, nuevoEstado, usuarioID uuid.UUID, motivo string) error {
	return r.p.WithTx(ctx, func(tx pgx.Tx) error {
		var anterior uuid.UUID
		err := tx.QueryRow(ctx,
			`SELECT estado_id FROM equipo WHERE id = $1 AND deleted_at IS NULL FOR UPDATE`,
			equipoID).Scan(&anterior)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}
		if anterior == nuevoEstado {
			return domain.ErrConflict
		}
		now := time.Now().UTC()
		if _, err := tx.Exec(ctx,
			`UPDATE equipo SET estado_id = $2, estado_desde = $3, updated_by = $4 WHERE id = $1`,
			equipoID, nuevoEstado, now, usuarioID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO equipo_estado_historial
			    (equipo_id, estado_anterior_id, estado_nuevo_id, usuario_id, motivo)
			VALUES ($1, $2, $3, $4, NULLIF($5,''))`,
			equipoID, anterior, nuevoEstado, usuarioID, motivo)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE alerta_evento
			SET resuelta_at = COALESCE(resuelta_at, NOW()),
			    resuelta_por = $2,
			    resolucion_motivo = 'cambio_estado_equipo',
			    pospuesta_hasta = NULL,
			    pospuesta_por = NULL
			WHERE equipo_id = $1
			  AND resuelta_at IS NULL`,
			equipoID, usuarioID)
		return err
	})
}

// EstadoHistorialEntry es una fila enriquecida (con nombres) del historial de
// estados. Se usa solo en lectura para pintar la timeline.
type EstadoHistorialEntry struct {
	ID                uuid.UUID  `json:"id"`
	Fecha             time.Time  `json:"fecha"`
	EstadoAnteriorID  *uuid.UUID `json:"estado_anterior_id,omitempty"`
	EstadoAnteriorNom string     `json:"estado_anterior,omitempty"`
	EstadoNuevoID     uuid.UUID  `json:"estado_nuevo_id"`
	EstadoNuevoNom    string     `json:"estado_nuevo"`
	EstadoNuevoColor  string     `json:"estado_nuevo_color,omitempty"`
	UsuarioID         uuid.UUID  `json:"usuario_id"`
	UsuarioNombre     string     `json:"usuario_nombre"`
	Motivo            string     `json:"motivo,omitempty"`
}

// ListEstadoHistorial trae el historial de estados de un equipo, más reciente
// primero.
func (r *EquipoRepo) ListEstadoHistorial(ctx context.Context, equipoID uuid.UUID) ([]EstadoHistorialEntry, error) {
	rows, err := r.p.Query(ctx, `
		SELECT h.id, h.fecha,
		       h.estado_anterior_id, COALESCE(ea.nombre, ''),
		       h.estado_nuevo_id, en.nombre, COALESCE(en.color, ''),
		       h.usuario_id, u.nombre,
		       COALESCE(h.motivo, '')
		FROM equipo_estado_historial h
		JOIN estado_operativo en ON en.id = h.estado_nuevo_id
		LEFT JOIN estado_operativo ea ON ea.id = h.estado_anterior_id
		JOIN usuario u ON u.id = h.usuario_id
		WHERE h.equipo_id = $1
		ORDER BY h.fecha DESC, h.id DESC`, equipoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EstadoHistorialEntry
	for rows.Next() {
		var e EstadoHistorialEntry
		if err := rows.Scan(&e.ID, &e.Fecha,
			&e.EstadoAnteriorID, &e.EstadoAnteriorNom,
			&e.EstadoNuevoID, &e.EstadoNuevoNom, &e.EstadoNuevoColor,
			&e.UsuarioID, &e.UsuarioNombre, &e.Motivo); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CambioEntry es una mutación a nivel de campo registrada en cambio_historial.
type CambioEntry struct {
	ID            uuid.UUID `json:"id"`
	Fecha         time.Time `json:"fecha"`
	Campo         string    `json:"campo"`
	ValorAnterior string    `json:"valor_anterior,omitempty"`
	ValorNuevo    string    `json:"valor_nuevo,omitempty"`
	UsuarioID     uuid.UUID `json:"usuario_id"`
	UsuarioNombre string    `json:"usuario_nombre"`
}

// ListCambios devuelve los cambios de campo registrados para el equipo.
func (r *EquipoRepo) ListCambios(ctx context.Context, equipoID uuid.UUID) ([]CambioEntry, error) {
	rows, err := r.p.Query(ctx, `
		SELECT c.id, c.fecha, c.campo,
		       COALESCE(c.valor_anterior, ''), COALESCE(c.valor_nuevo, ''),
		       c.usuario_id, u.nombre
		FROM cambio_historial c
		JOIN usuario u ON u.id = c.usuario_id
		WHERE c.entidad = 'EQUIPO' AND c.entidad_id = $1
		ORDER BY c.fecha DESC, c.id DESC`, equipoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CambioEntry
	for rows.Next() {
		var e CambioEntry
		if err := rows.Scan(&e.ID, &e.Fecha, &e.Campo,
			&e.ValorAnterior, &e.ValorNuevo, &e.UsuarioID, &e.UsuarioNombre); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
