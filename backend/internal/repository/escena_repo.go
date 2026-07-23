package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gitlab.com/syncbox/backend/internal/domain"
)

// EscenaRepo gestiona las tablas `escena` y `escena_instancia`.
type EscenaRepo struct{ p *Pool }

// NewEscenaRepo construye el repositorio.
func NewEscenaRepo(p *Pool) *EscenaRepo { return &EscenaRepo{p: p} }

// --- ESCENA ------------------------------------------------------------------

const escenaCols = `
	id, nombre, descripcion, activo, nodo_id,
	luz_activa, luz_intensidad, luz_color,
	luz_pos_x, luz_pos_y, luz_pos_z,
	luz_target_x, luz_target_y, luz_target_z,
	luz_angulo, luz_penumbra, luz_distancia, luz_auto_target,
	created_at, updated_at, created_by, updated_by`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEscena(row rowScanner, e *domain.Escena) error {
	return row.Scan(
		&e.ID, &e.Nombre, &e.Descripcion, &e.Activo, &e.NodoID,
		&e.Iluminacion.Activa, &e.Iluminacion.Intensidad, &e.Iluminacion.Color,
		&e.Iluminacion.PosX, &e.Iluminacion.PosY, &e.Iluminacion.PosZ,
		&e.Iluminacion.TargetX, &e.Iluminacion.TargetY, &e.Iluminacion.TargetZ,
		&e.Iluminacion.Angulo, &e.Iluminacion.Penumbra, &e.Iluminacion.Distancia, &e.Iluminacion.AutoTarget,
		&e.CreatedAt, &e.UpdatedAt, &e.CreatedBy, &e.UpdatedBy,
	)
}

func scanLabSesion(row rowScanner, s *domain.LabSesion) error {
	return row.Scan(
		&s.ID, &s.EscenaID, &s.UsuarioID,
		&s.IniciadaAt, &s.CerradaAt, &s.UltimaActividadAt, &s.CierreMotivo,
	)
}

type LabAuditFilters struct {
	Search string
	Desde  *time.Time
	Hasta  *time.Time
	Estado string
	Limit  int
	Offset int
}

// List devuelve las escenas no borradas, opcionalmente solo activas.
func (r *EscenaRepo) List(ctx context.Context, soloActivas bool) ([]domain.Escena, error) {
	q := `SELECT ` + escenaCols + `
	      FROM escena
	      WHERE deleted_at IS NULL`
	if soloActivas {
		q += ` AND activo = TRUE`
	}
	q += ` ORDER BY nombre`
	rows, err := r.p.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Escena
	for rows.Next() {
		var e domain.Escena
		if err := scanEscena(rows, &e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// FindByID recupera una escena no borrada.
func (r *EscenaRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Escena, error) {
	var e domain.Escena
	err := scanEscena(r.p.QueryRow(ctx, `
		SELECT `+escenaCols+`
		FROM escena WHERE id = $1 AND deleted_at IS NULL`, id,
	), &e)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &e, nil
}

// Create inserta una escena.
func (r *EscenaRepo) Create(ctx context.Context, e *domain.Escena, actor uuid.UUID) error {
	return MapPgError(r.p.QueryRow(ctx, `
		INSERT INTO escena (nombre, descripcion, activo, nodo_id, created_by, updated_by)
		VALUES ($1, $2, TRUE, $3, $4, $4)
		RETURNING id, created_at, updated_at`,
		e.Nombre, e.Descripcion, e.NodoID, actor,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt))
}

// Update parcial.
func (r *EscenaRepo) Update(ctx context.Context, id uuid.UUID, nombre, descripcion *string, activo *bool, nodoID *uuid.UUID, actor uuid.UUID) error {
	tag, err := r.p.Exec(ctx, `
		UPDATE escena SET
			nombre      = COALESCE($2, nombre),
			descripcion = COALESCE($3, descripcion),
			activo      = COALESCE($4, activo),
			nodo_id     = COALESCE($5, nodo_id),
			updated_at  = now(),
			updated_by  = $6
		WHERE id = $1 AND deleted_at IS NULL`,
		id, nombre, descripcion, activo, nodoID, actor)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UpdateLighting actualiza la configuracion de foco del laboratorio.
func (r *EscenaRepo) UpdateLighting(ctx context.Context, id uuid.UUID, light domain.EscenaLight, actor uuid.UUID) error {
	tag, err := r.p.Exec(ctx, `
		UPDATE escena SET
			luz_activa      = $2,
			luz_intensidad  = $3,
			luz_color       = $4,
			luz_pos_x       = $5,
			luz_pos_y       = $6,
			luz_pos_z       = $7,
			luz_target_x    = $8,
			luz_target_y    = $9,
			luz_target_z    = $10,
			luz_angulo      = $11,
			luz_penumbra    = $12,
			luz_distancia   = $13,
			luz_auto_target = $14,
			updated_at      = now(),
			updated_by      = $15
		WHERE id = $1 AND deleted_at IS NULL`,
		id,
		light.Activa, light.Intensidad, light.Color,
		light.PosX, light.PosY, light.PosZ,
		light.TargetX, light.TargetY, light.TargetZ,
		light.Angulo, light.Penumbra, light.Distancia, light.AutoTarget,
		actor)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// CreateLabSesion registra una nueva entrada al modo laboratorio.
func (r *EscenaRepo) CreateLabSesion(ctx context.Context, escenaID, actor uuid.UUID) (*domain.LabSesion, error) {
	var s domain.LabSesion
	err := scanLabSesion(r.p.QueryRow(ctx, `
		INSERT INTO lab_sesion (escena_id, usuario_id)
		SELECT id, $2
		FROM escena
		WHERE id = $1 AND deleted_at IS NULL
		RETURNING id, escena_id, usuario_id, iniciada_at, cerrada_at, ultima_actividad_at, cierre_motivo`,
		escenaID, actor,
	), &s)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, MapPgError(err)
	}
	return &s, nil
}

// CloseLabSesion marca una sesion como cerrada. Si el navegador se cierra sin
// avisar, los snapshots previos quedan igualmente disponibles por
// ultima_actividad_at.
func (r *EscenaRepo) CloseLabSesion(ctx context.Context, escenaID, sesionID, actor uuid.UUID, motivo string) (*domain.LabSesion, error) {
	var s domain.LabSesion
	err := scanLabSesion(r.p.QueryRow(ctx, `
		UPDATE lab_sesion SET
			cerrada_at = COALESCE(cerrada_at, now()),
			ultima_actividad_at = now(),
			cierre_motivo = $4
		WHERE id = $1 AND escena_id = $2 AND usuario_id = $3
		RETURNING id, escena_id, usuario_id, iniciada_at, cerrada_at, ultima_actividad_at, cierre_motivo`,
		sesionID, escenaID, actor, motivo,
	), &s)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, MapPgError(err)
	}
	return &s, nil
}

// SoftDelete marca la escena como eliminada.
func (r *EscenaRepo) SoftDelete(ctx context.Context, id uuid.UUID, actor uuid.UUID) error {
	tag, err := r.p.Exec(ctx, `
		UPDATE escena SET deleted_at = now(), activo = FALSE, updated_at = now(), updated_by = $2
		WHERE id = $1 AND deleted_at IS NULL`, id, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// --- INSTANCIA ---------------------------------------------------------------

const instanciaCols = `
	id, escena_id, equipo_origen_id, orden,
	nombre_snapshot, fabricante_snapshot, modelo_snapshot, categoria_snapshot,
	pos_x, pos_y, pos_z, escala,
	rot_x, rot_y, rot_z,
	pos_inicial_x, pos_inicial_y, pos_inicial_z, escala_inicial,
	rot_inicial_x, rot_inicial_y, rot_inicial_z,
	created_at, updated_at`

func scanInstancia(rows pgx.Row, i *domain.EscenaInstancia) error {
	return rows.Scan(
		&i.ID, &i.EscenaID, &i.EquipoOrigenID, &i.Orden,
		&i.NombreSnapshot, &i.FabricanteSnapshot, &i.ModeloSnapshot, &i.CategoriaSnapshot,
		&i.PosX, &i.PosY, &i.PosZ, &i.Escala,
		&i.RotX, &i.RotY, &i.RotZ,
		&i.PosInicialX, &i.PosInicialY, &i.PosInicialZ, &i.EscalaInicial,
		&i.RotInicialX, &i.RotInicialY, &i.RotInicialZ,
		&i.CreatedAt, &i.UpdatedAt,
	)
}

// ListInstancias devuelve las instancias de una escena ordenadas por orden.
func (r *EscenaRepo) ListInstancias(ctx context.Context, escenaID uuid.UUID) ([]domain.EscenaInstancia, error) {
	rows, err := r.p.Query(ctx,
		`SELECT `+instanciaCols+` FROM escena_instancia WHERE escena_id = $1 ORDER BY orden`,
		escenaID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.EscenaInstancia
	for rows.Next() {
		var i domain.EscenaInstancia
		if err := scanInstancia(rows, &i); err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

// FindInstancia recupera una instancia.
func (r *EscenaRepo) FindInstancia(ctx context.Context, escenaID, instanciaID uuid.UUID) (*domain.EscenaInstancia, error) {
	var i domain.EscenaInstancia
	err := scanInstancia(r.p.QueryRow(ctx,
		`SELECT `+instanciaCols+` FROM escena_instancia WHERE id = $1 AND escena_id = $2`,
		instanciaID, escenaID), &i)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &i, nil
}

// CreateInstancia inserta una nueva instancia. El orden se autoasigna como
// MAX(orden)+1 dentro de la misma escena para garantizar el display
// "nombre — modelo escena X". Los campos pos_inicial / escala_inicial se
// copian de los valores actuales recibidos.
func (r *EscenaRepo) CreateInstancia(ctx context.Context, i *domain.EscenaInstancia, actor uuid.UUID, sesionID *uuid.UUID) error {
	return r.p.WithTx(ctx, func(tx pgx.Tx) error {
		err := tx.QueryRow(ctx, `
			INSERT INTO escena_instancia (
				escena_id, equipo_origen_id, orden,
				nombre_snapshot, fabricante_snapshot, modelo_snapshot, categoria_snapshot,
				pos_x, pos_y, pos_z, escala,
				rot_x, rot_y, rot_z,
				pos_inicial_x, pos_inicial_y, pos_inicial_z, escala_inicial,
				rot_inicial_x, rot_inicial_y, rot_inicial_z
			) VALUES (
				$1, $2,
				COALESCE((SELECT MAX(orden) FROM escena_instancia WHERE escena_id = $1), 0) + 1,
				$3, $4, $5, $6,
				$7, $8, $9, $10,
				$11, $12, $13,
				$7, $8, $9, $10,
				$11, $12, $13
			) RETURNING id, orden, created_at, updated_at`,
			i.EscenaID, i.EquipoOrigenID,
			i.NombreSnapshot, i.FabricanteSnapshot, i.ModeloSnapshot, i.CategoriaSnapshot,
			i.PosX, i.PosY, i.PosZ, i.Escala,
			i.RotX, i.RotY, i.RotZ,
		).Scan(&i.ID, &i.Orden, &i.CreatedAt, &i.UpdatedAt)
		if err != nil {
			return MapPgError(err)
		}
		i.PosInicialX, i.PosInicialY, i.PosInicialZ = i.PosX, i.PosY, i.PosZ
		i.EscalaInicial = i.Escala
		i.RotInicialX, i.RotInicialY, i.RotInicialZ = i.RotX, i.RotY, i.RotZ

		var sid any
		if sesionID != nil {
			sid = *sesionID
		}
		tag, err := tx.Exec(ctx, `
			INSERT INTO lab_audit_event (
				lab_sesion_id, escena_id, instancia_id, usuario_id, event_type,
				equipo_origen_id, nombre_snapshot, fabricante_snapshot, modelo_snapshot, categoria_snapshot,
				pos_x, pos_y, pos_z, escala, rot_x, rot_y, rot_z
			)
			SELECT
				$1::uuid, $2, $3, $4, 'add',
				$5, $6, $7, $8, $9,
				$10, $11, $12, $13, $14, $15, $16
			WHERE $1::uuid IS NULL OR EXISTS (
				SELECT 1
				FROM lab_sesion
				WHERE id = $1
				  AND escena_id = $2
				  AND usuario_id = $4
				  AND cerrada_at IS NULL
			)`,
			sid, i.EscenaID, i.ID, actor,
			i.EquipoOrigenID, i.NombreSnapshot, i.FabricanteSnapshot, i.ModeloSnapshot, i.CategoriaSnapshot,
			i.PosX, i.PosY, i.PosZ, i.Escala, i.RotX, i.RotY, i.RotZ)
		if err != nil {
			return MapPgError(err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

// UpdateInstanciaTransform actualiza posicion, escala y rotacion.
func (r *EscenaRepo) UpdateInstanciaTransform(ctx context.Context, escenaID, instanciaID uuid.UUID,
	posX, posY, posZ, escala, rotX, rotY, rotZ *float64) error {
	tag, err := r.p.Exec(ctx, `
		UPDATE escena_instancia SET
			pos_x      = COALESCE($3, pos_x),
			pos_y      = COALESCE($4, pos_y),
			pos_z      = COALESCE($5, pos_z),
			escala     = COALESCE($6, escala),
			rot_x      = COALESCE($7, rot_x),
			rot_y      = COALESCE($8, rot_y),
			rot_z      = COALESCE($9, rot_z),
			updated_at = now()
		WHERE id = $1 AND escena_id = $2`,
		instanciaID, escenaID, posX, posY, posZ, escala, rotX, rotY, rotZ)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UpsertLabSesionInstancia guarda el ultimo transform de una instancia dentro
// de una sesion activa y agrega un evento append-only para auditoria.
func (r *EscenaRepo) UpsertLabSesionInstancia(ctx context.Context, escenaID, sesionID, actor uuid.UUID, eventType string, snap domain.LabSesionInstancia) error {
	tag, err := r.p.Exec(ctx, `
		WITH valid AS (
			SELECT s.id AS lab_sesion_id,
			       s.escena_id,
			       i.id AS instancia_id,
			       i.equipo_origen_id,
			       i.nombre_snapshot,
			       i.fabricante_snapshot,
			       i.modelo_snapshot,
			       i.categoria_snapshot
			FROM lab_sesion s
			JOIN escena_instancia i ON i.id = $4 AND i.escena_id = s.escena_id
			WHERE s.id = $1
			  AND s.escena_id = $2
			  AND s.usuario_id = $3
			  AND s.cerrada_at IS NULL
		), snap AS (
			INSERT INTO lab_sesion_instancia (
				lab_sesion_id, instancia_id,
				pos_x, pos_y, pos_z, escala,
				rot_x, rot_y, rot_z
			)
			SELECT lab_sesion_id, instancia_id, $5, $6, $7, $8, $9, $10, $11
			FROM valid
			ON CONFLICT (lab_sesion_id, instancia_id) DO UPDATE SET
				pos_x = EXCLUDED.pos_x,
				pos_y = EXCLUDED.pos_y,
				pos_z = EXCLUDED.pos_z,
				escala = EXCLUDED.escala,
				rot_x = EXCLUDED.rot_x,
				rot_y = EXCLUDED.rot_y,
				rot_z = EXCLUDED.rot_z,
				updated_at = now()
			RETURNING lab_sesion_id, instancia_id
		)
		INSERT INTO lab_audit_event (
			lab_sesion_id, escena_id, instancia_id, usuario_id, event_type,
			equipo_origen_id, nombre_snapshot, fabricante_snapshot, modelo_snapshot, categoria_snapshot,
			pos_x, pos_y, pos_z, escala, rot_x, rot_y, rot_z
		)
		SELECT
			v.lab_sesion_id, v.escena_id, v.instancia_id, $3, $12,
			v.equipo_origen_id, v.nombre_snapshot, v.fabricante_snapshot, v.modelo_snapshot, v.categoria_snapshot,
			$5, $6, $7, $8, $9, $10, $11
		FROM valid v
		JOIN snap s ON s.lab_sesion_id = v.lab_sesion_id AND s.instancia_id = v.instancia_id`,
		sesionID, escenaID, actor, snap.InstanciaID,
		snap.PosX, snap.PosY, snap.PosZ, snap.Escala,
		snap.RotX, snap.RotY, snap.RotZ, eventType,
	)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	_, err = r.p.Exec(ctx, `
		UPDATE lab_sesion
		SET ultima_actividad_at = now()
		WHERE id = $1 AND escena_id = $2 AND usuario_id = $3 AND cerrada_at IS NULL`,
		sesionID, escenaID, actor,
	)
	return MapPgError(err)
}

// RecordLabAuditEvent registra eventos que no necesariamente ocurren dentro de
// una sesion interactiva, como agregar o quitar un objeto desde la gestion.
func (r *EscenaRepo) RecordLabAuditEvent(ctx context.Context, escenaID uuid.UUID, sesionID *uuid.UUID, actor uuid.UUID, eventType string, inst *domain.EscenaInstancia) error {
	if inst == nil {
		return nil
	}
	var sid any
	if sesionID != nil {
		sid = *sesionID
	}
	tag, err := r.p.Exec(ctx, `
		INSERT INTO lab_audit_event (
			lab_sesion_id, escena_id, instancia_id, usuario_id, event_type,
			equipo_origen_id, nombre_snapshot, fabricante_snapshot, modelo_snapshot, categoria_snapshot,
			pos_x, pos_y, pos_z, escala, rot_x, rot_y, rot_z
		)
		SELECT
			$1::uuid, $2, $3, $4, $5,
			$6, $7, $8, $9, $10,
			$11, $12, $13, $14, $15, $16, $17
		WHERE $1::uuid IS NULL OR EXISTS (
			SELECT 1
			FROM lab_sesion
			WHERE id = $1
			  AND escena_id = $2
			  AND usuario_id = $4
			  AND cerrada_at IS NULL
		)`,
		sid, escenaID, inst.ID, actor, eventType,
		inst.EquipoOrigenID, inst.NombreSnapshot, inst.FabricanteSnapshot, inst.ModeloSnapshot, inst.CategoriaSnapshot,
		inst.PosX, inst.PosY, inst.PosZ, inst.Escala, inst.RotX, inst.RotY, inst.RotZ,
	)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// FindLatestSessionSnapshot busca el ultimo transform historico de una
// instancia, excluyendo opcionalmente la sesion actual.
func (r *EscenaRepo) FindLatestSessionSnapshot(ctx context.Context, escenaID, instanciaID uuid.UUID, excludeSesionID *uuid.UUID) (*domain.LabSesionInstancia, error) {
	var exclude any
	if excludeSesionID != nil {
		exclude = *excludeSesionID
	}
	var snap domain.LabSesionInstancia
	err := r.p.QueryRow(ctx, `
		SELECT lsi.lab_sesion_id, lsi.instancia_id,
		       lsi.pos_x, lsi.pos_y, lsi.pos_z, lsi.escala,
		       lsi.rot_x, lsi.rot_y, lsi.rot_z, lsi.updated_at
		FROM lab_sesion_instancia lsi
		JOIN lab_sesion s ON s.id = lsi.lab_sesion_id
		WHERE s.escena_id = $1
		  AND lsi.instancia_id = $2
		  AND ($3::uuid IS NULL OR s.id <> $3)
		ORDER BY COALESCE(s.cerrada_at, s.ultima_actividad_at, s.iniciada_at) DESC,
		         lsi.updated_at DESC
		LIMIT 1`,
		escenaID, instanciaID, exclude,
	).Scan(
		&snap.LabSesionID, &snap.InstanciaID,
		&snap.PosX, &snap.PosY, &snap.PosZ, &snap.Escala,
		&snap.RotX, &snap.RotY, &snap.RotZ, &snap.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, MapPgError(err)
	}
	return &snap, nil
}

// ListLabAudit devuelve eventos historicos append-only de transform/alta/baja
// por instancia, enriquecidos con usuario e informacion textual del equipo.
func (r *EscenaRepo) ListLabAudit(ctx context.Context, escenaID uuid.UUID, f LabAuditFilters) ([]domain.LabAuditEntry, int, error) {
	if f.Limit <= 0 || f.Limit > 200 {
		f.Limit = 80
	}
	if f.Offset < 0 {
		f.Offset = 0
	}

	rows, err := r.p.Query(ctx, `
		SELECT
			COALESCE(lae.lab_sesion_id::text, ''),
			lae.instancia_id,
			lae.escena_id,
			lae.usuario_id,
			lae.event_type,
			COALESCE(u.nombre, ''),
			COALESCE(u.correo, ''),
			COALESCE(s.iniciada_at, lae.fecha),
			s.cerrada_at,
			COALESCE(s.ultima_actividad_at, lae.fecha),
			COALESCE(s.cierre_motivo, ''),
			lae.fecha,
			lae.equipo_origen_id,
			COALESCE(lae.nombre_snapshot, ''),
			COALESCE(lae.fabricante_snapshot, ''),
			COALESCE(lae.modelo_snapshot, ''),
			COALESCE(lae.categoria_snapshot, ''),
			lae.pos_x,
			lae.pos_y,
			lae.pos_z,
			lae.escala,
			lae.rot_x,
			lae.rot_y,
			lae.rot_z,
			COUNT(*) OVER() AS total_count
		FROM lab_audit_event lae
		JOIN escena e ON e.id = lae.escena_id AND e.deleted_at IS NULL
		LEFT JOIN lab_sesion s ON s.id = lae.lab_sesion_id
		LEFT JOIN usuario u ON u.id = lae.usuario_id
		WHERE lae.escena_id = $1
		  AND (
			$2 = ''
			OR COALESCE(lae.nombre_snapshot, '') ILIKE '%' || $2 || '%'
			OR COALESCE(lae.fabricante_snapshot, '') ILIKE '%' || $2 || '%'
			OR COALESCE(lae.modelo_snapshot, '') ILIKE '%' || $2 || '%'
			OR COALESCE(lae.categoria_snapshot, '') ILIKE '%' || $2 || '%'
			OR COALESCE(lae.event_type, '') ILIKE '%' || $2 || '%'
			OR COALESCE(u.nombre, '') ILIKE '%' || $2 || '%'
			OR COALESCE(u.correo, '') ILIKE '%' || $2 || '%'
			OR lae.instancia_id::text ILIKE '%' || $2 || '%'
			OR COALESCE(lae.lab_sesion_id::text, '') ILIKE '%' || $2 || '%'
		  )
		  AND ($3::timestamptz IS NULL OR lae.fecha >= $3)
		  AND ($4::timestamptz IS NULL OR lae.fecha < $4)
		  AND (
			$5 = ''
			OR ($5 = 'abierta' AND s.cerrada_at IS NULL)
			OR ($5 = 'cerrada' AND s.cerrada_at IS NOT NULL)
		  )
		ORDER BY lae.fecha DESC, lae.id DESC
		LIMIT $6 OFFSET $7`,
		escenaID, f.Search, f.Desde, f.Hasta, f.Estado, f.Limit, f.Offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []domain.LabAuditEntry
	total := 0
	for rows.Next() {
		var e domain.LabAuditEntry
		var rowTotal int
		if err := rows.Scan(
			&e.LabSesionID,
			&e.InstanciaID,
			&e.EscenaID,
			&e.UsuarioID,
			&e.EventType,
			&e.UsuarioNombre,
			&e.UsuarioCorreo,
			&e.SesionIniciadaAt,
			&e.SesionCerradaAt,
			&e.SesionUltimaActividad,
			&e.CierreMotivo,
			&e.Fecha,
			&e.EquipoOrigenID,
			&e.NombreSnapshot,
			&e.FabricanteSnapshot,
			&e.ModeloSnapshot,
			&e.CategoriaSnapshot,
			&e.PosX,
			&e.PosY,
			&e.PosZ,
			&e.Escala,
			&e.RotX,
			&e.RotY,
			&e.RotZ,
			&rowTotal,
		); err != nil {
			return nil, 0, err
		}
		total = rowTotal
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

// DeleteInstancia elimina una instancia y registra el evento de baja en la
// misma transaccion para preservar la auditoria.
func (r *EscenaRepo) DeleteInstancia(ctx context.Context, escenaID, instanciaID, actor uuid.UUID) error {
	return r.p.WithTx(ctx, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			INSERT INTO lab_audit_event (
				lab_sesion_id, escena_id, instancia_id, usuario_id, event_type,
				equipo_origen_id, nombre_snapshot, fabricante_snapshot, modelo_snapshot, categoria_snapshot,
				pos_x, pos_y, pos_z, escala, rot_x, rot_y, rot_z
			)
			SELECT
				NULL, escena_id, id, $3, 'remove',
				equipo_origen_id, nombre_snapshot, fabricante_snapshot, modelo_snapshot, categoria_snapshot,
				pos_x, pos_y, pos_z, escala, rot_x, rot_y, rot_z
			FROM escena_instancia
			WHERE id = $1 AND escena_id = $2`,
			instanciaID, escenaID, actor)
		if err != nil {
			return MapPgError(err)
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}

		tag, err = tx.Exec(ctx,
			`DELETE FROM escena_instancia WHERE id = $1 AND escena_id = $2`,
			instanciaID, escenaID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}

		if _, err := tx.Exec(ctx, `
			WITH ranked AS (
				SELECT id, ROW_NUMBER() OVER (ORDER BY orden, created_at, id) AS rn
				FROM escena_instancia
				WHERE escena_id = $1
			)
			UPDATE escena_instancia i
			SET orden = -ranked.rn
			FROM ranked
			WHERE i.id = ranked.id`, escenaID); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			UPDATE escena_instancia
			SET orden = -orden
			WHERE escena_id = $1 AND orden < 0`, escenaID)
		return err
	})
}
