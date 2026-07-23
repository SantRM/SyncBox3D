package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gitlab.com/syncbox/backend/internal/domain"
)

// AlertaRepo gestiona la generacion, configuracion y ciclo de vida de alertas.
type AlertaRepo struct{ p *Pool }

// NewAlertaRepo construye el repositorio.
func NewAlertaRepo(p *Pool) *AlertaRepo { return &AlertaRepo{p: p} }

// VencidoCandidato representa un equipo cuyo tiempo en estado supera el umbral.
type VencidoCandidato struct {
	EquipoID     uuid.UUID
	EstadoID     uuid.UUID
	EstadoNombre string
	DiasUmbral   int
}

type AlertaListFilters struct {
	Estado string
	Search string
	Limit  int
	Offset int
}

func estadoSeguro(nombre string) bool {
	n := strings.ToLower(strings.TrimSpace(nombre))
	return n == "disponible" || n == "en uso"
}

// FindCandidatos recupera equipos que llevan en su estado un numero de dias
// igual o superior al umbral activo. Los estados seguros nunca generan alerta.
func (r *AlertaRepo) FindCandidatos(ctx context.Context) ([]VencidoCandidato, error) {
	rows, err := r.p.Query(ctx, `
		SELECT e.id, e.estado_id, eo.nombre, COALESCE(ac.dias_umbral, 1)
		FROM equipo e
		JOIN estado_operativo eo ON eo.id = e.estado_id
		LEFT JOIN alerta_config ac ON ac.estado_id = e.estado_id
		WHERE e.deleted_at IS NULL
		  AND e.activo = TRUE
		  AND eo.activo = TRUE
		  AND COALESCE(ac.activa, TRUE) = TRUE
		  AND lower(eo.nombre) NOT IN ('disponible', 'en uso')
		  AND NOW() - e.estado_desde >= MAKE_INTERVAL(days => COALESCE(ac.dias_umbral, 1))
		  AND NOT EXISTS (
		      SELECT 1
		      FROM alerta_evento ae
		      WHERE ae.equipo_id = e.id
		        AND ae.resuelta_at IS NULL
		  )`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []VencidoCandidato
	for rows.Next() {
		var c VencidoCandidato
		if err := rows.Scan(&c.EquipoID, &c.EstadoID, &c.EstadoNombre, &c.DiasUmbral); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ResolveObsoleteAuto cierra alertas que ya no aplican: el equipo cambio de
// estado, entro a un estado seguro, fue desactivado/borrado o se apago config.
func (r *AlertaRepo) ResolveObsoleteAuto(ctx context.Context) (int, error) {
	tag, err := r.p.Exec(ctx, `
		UPDATE alerta_evento ae
		SET resuelta_at = now(),
		    resolucion_motivo = CASE
		        WHEN e.id IS NULL OR e.deleted_at IS NOT NULL OR e.activo = FALSE THEN 'equipo_no_activo'
		        WHEN e.estado_id <> ae.estado_id THEN 'cambio_estado_equipo'
		        WHEN lower(eo.nombre) IN ('disponible', 'en uso') THEN 'estado_seguro'
		        WHEN COALESCE(ac.activa, TRUE) = FALSE THEN 'configuracion_desactivada'
		        ELSE 'auto'
		    END
		FROM equipo e
		LEFT JOIN estado_operativo eo ON eo.id = e.estado_id
		LEFT JOIN alerta_config ac ON ac.estado_id = e.estado_id
		WHERE ae.equipo_id = e.id
		  AND ae.resuelta_at IS NULL
		  AND (
		       e.deleted_at IS NOT NULL
		       OR e.activo = FALSE
		       OR e.estado_id <> ae.estado_id
		       OR lower(eo.nombre) IN ('disponible', 'en uso')
		       OR COALESCE(ac.activa, TRUE) = FALSE
		  )`)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

// ResolveActiveForEquipo resuelve cualquier alerta activa del equipo. Se usa
// despues de cambios manuales de estado.
func (r *AlertaRepo) ResolveActiveForEquipo(ctx context.Context, equipoID, actor uuid.UUID, motivo string) error {
	if strings.TrimSpace(motivo) == "" {
		motivo = "cambio_estado_equipo"
	}
	_, err := r.p.Exec(ctx, `
		UPDATE alerta_evento
		SET resuelta_at = COALESCE(resuelta_at, now()),
		    resuelta_por = $2,
		    resolucion_motivo = $3,
		    pospuesta_hasta = NULL,
		    pospuesta_por = NULL
		WHERE equipo_id = $1
		  AND resuelta_at IS NULL`,
		equipoID, actor, motivo,
	)
	return MapPgError(err)
}

// CreateEvento inserta una alerta activa. La unicidad parcial por equipo evita
// duplicados mientras exista una pendiente/pospuesta sin resolver.
func (r *AlertaRepo) CreateEvento(ctx context.Context, equipoID, estadoID uuid.UUID) (bool, error) {
	tag, err := r.p.Exec(ctx, `
		INSERT INTO alerta_evento (equipo_id, estado_id)
		VALUES ($1, $2)
		ON CONFLICT DO NOTHING`, equipoID, estadoID)
	if err != nil {
		return false, MapPgError(err)
	}
	return tag.RowsAffected() > 0, nil
}

func scanAlerta(rows rowScanner, a *domain.AlertaEvento) error {
	return rows.Scan(
		&a.ID,
		&a.EquipoID,
		&a.EstadoID,
		&a.GeneradaAt,
		&a.VistaAt,
		&a.VistaPor,
		&a.ResueltaAt,
		&a.ResueltaPor,
		&a.ResolucionMotivo,
		&a.PospuestaHasta,
		&a.PospuestaPor,
		&a.UpdatedAt,
		&a.EquipoNombre,
		&a.EstadoNombre,
		&a.EstadoColor,
		&a.EstadoDesde,
		&a.DiasUmbral,
		&a.DiasEnEstado,
	)
}

const alertaSelect = `
	ae.id,
	ae.equipo_id,
	ae.estado_id,
	ae.generada_at,
	ae.vista_at,
	ae.vista_por,
	ae.resuelta_at,
	ae.resuelta_por,
	COALESCE(ae.resolucion_motivo, ''),
	ae.pospuesta_hasta,
	ae.pospuesta_por,
	ae.updated_at,
	e.nombre,
	eo.nombre,
	COALESCE(eo.color, ''),
	e.estado_desde,
	COALESCE(ac.dias_umbral, 0),
	GREATEST(FLOOR(EXTRACT(EPOCH FROM (NOW() - e.estado_desde)) / 86400)::int, 0)`

// ListPendientes lista alertas activas; si dueOnly es true solo devuelve las
// que deben mostrarse en popup porque no estan pospuestas o ya vencio el plazo.
func (r *AlertaRepo) ListPendientes(ctx context.Context, dueOnly bool) ([]domain.AlertaEvento, error) {
	q := `
		SELECT ` + alertaSelect + `
		FROM alerta_evento ae
		JOIN equipo e ON e.id = ae.equipo_id
		JOIN estado_operativo eo ON eo.id = ae.estado_id
		LEFT JOIN alerta_config ac ON ac.estado_id = ae.estado_id
		WHERE ae.resuelta_at IS NULL`
	if dueOnly {
		q += ` AND (ae.pospuesta_hasta IS NULL OR ae.pospuesta_hasta <= NOW())`
	}
	q += ` ORDER BY COALESCE(ae.pospuesta_hasta, ae.generada_at) ASC, ae.generada_at ASC`

	rows, err := r.p.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AlertaEvento
	for rows.Next() {
		var a domain.AlertaEvento
		if err := scanAlerta(rows, &a); err != nil {
			return nil, err
		}
		a.Razon = alertaRazon(a)
		out = append(out, a)
	}
	return out, rows.Err()
}

// ListEventos devuelve alertas filtradas para la vista de gestion/historial.
func (r *AlertaRepo) ListEventos(ctx context.Context, f AlertaListFilters) ([]domain.AlertaEvento, error) {
	if f.Limit <= 0 || f.Limit > 300 {
		f.Limit = 100
	}
	if f.Offset < 0 {
		f.Offset = 0
	}
	whereEstado := ``
	if f.Estado == "pendiente" {
		whereEstado = ` AND ae.resuelta_at IS NULL`
	} else if f.Estado == "resuelta" {
		whereEstado = ` AND ae.resuelta_at IS NOT NULL`
	}

	rows, err := r.p.Query(ctx, `
		SELECT `+alertaSelect+`
		FROM alerta_evento ae
		JOIN equipo e ON e.id = ae.equipo_id
		JOIN estado_operativo eo ON eo.id = ae.estado_id
		LEFT JOIN alerta_config ac ON ac.estado_id = ae.estado_id
		WHERE (
		    $1 = ''
		    OR e.nombre ILIKE '%' || $1 || '%'
		    OR eo.nombre ILIKE '%' || $1 || '%'
		    OR ae.id::text ILIKE '%' || $1 || '%'
		    OR ae.equipo_id::text ILIKE '%' || $1 || '%'
		)`+whereEstado+`
		ORDER BY COALESCE(ae.resuelta_at, ae.generada_at) DESC, ae.generada_at DESC
		LIMIT $2 OFFSET $3`,
		f.Search, f.Limit, f.Offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AlertaEvento
	for rows.Next() {
		var a domain.AlertaEvento
		if err := scanAlerta(rows, &a); err != nil {
			return nil, err
		}
		a.Razon = alertaRazon(a)
		out = append(out, a)
	}
	return out, rows.Err()
}

func alertaRazon(a domain.AlertaEvento) string {
	if a.DiasUmbral > 0 {
		return fmt.Sprintf(
			"%s lleva %d dias en estado %s; supera el umbral configurado de %d dias.",
			a.EquipoNombre,
			a.DiasEnEstado,
			a.EstadoNombre,
			a.DiasUmbral,
		)
	}
	return fmt.Sprintf("%s tiene una alerta pendiente por estado %s.", a.EquipoNombre, a.EstadoNombre)
}

// MarkVista se conserva por compatibilidad y equivale a resolver manualmente.
func (r *AlertaRepo) MarkVista(ctx context.Context, alertaID, usuarioID uuid.UUID) error {
	return r.Resolve(ctx, alertaID, usuarioID, "vista_resuelta")
}

// Resolve marca una alerta como resuelta.
func (r *AlertaRepo) Resolve(ctx context.Context, alertaID, usuarioID uuid.UUID, motivo string) error {
	if strings.TrimSpace(motivo) == "" {
		motivo = "manual"
	}
	tag, err := r.p.Exec(ctx, `
		UPDATE alerta_evento
		SET resuelta_at = COALESCE(resuelta_at, NOW()),
		    resuelta_por = $2,
		    resolucion_motivo = $3,
		    pospuesta_hasta = NULL,
		    pospuesta_por = NULL
		WHERE id = $1
		  AND resuelta_at IS NULL`, alertaID, usuarioID, motivo)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ResolveWithSafeState resuelve una alerta cambiando primero el equipo a un
// estado que no genera alertas. Prioriza "Disponible" y usa "En uso" como
// respaldo. Todo ocurre en una sola transaccion para que la alerta no quede
// resuelta si el estado del equipo no cambia.
func (r *AlertaRepo) ResolveWithSafeState(ctx context.Context, alertaID, usuarioID uuid.UUID) error {
	return r.p.WithTx(ctx, func(tx pgx.Tx) error {
		var equipoID uuid.UUID
		var estadoActual uuid.UUID
		if err := tx.QueryRow(ctx, `
			SELECT ae.equipo_id, e.estado_id
			FROM alerta_evento ae
			JOIN equipo e ON e.id = ae.equipo_id
			WHERE ae.id = $1
			  AND ae.resuelta_at IS NULL
			  AND e.deleted_at IS NULL
			  AND e.activo = TRUE
			FOR UPDATE OF ae, e`,
			alertaID,
		).Scan(&equipoID, &estadoActual); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrNotFound
			}
			return err
		}

		var safeEstado uuid.UUID
		err := tx.QueryRow(ctx, `
			SELECT id
			FROM estado_operativo
			WHERE activo = TRUE AND lower(nombre) = 'disponible'
			ORDER BY orden, nombre
			LIMIT 1`).Scan(&safeEstado)
		if errors.Is(err, pgx.ErrNoRows) {
			err = tx.QueryRow(ctx, `
				SELECT id
				FROM estado_operativo
				WHERE activo = TRUE AND lower(nombre) = 'en uso'
				ORDER BY orden, nombre
				LIMIT 1`).Scan(&safeEstado)
		}
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return domain.ErrInvalidInput
			}
			return err
		}

		if estadoActual != safeEstado {
			now := time.Now().UTC()
			if _, err := tx.Exec(ctx, `
				UPDATE equipo
				SET estado_id = $2,
				    estado_desde = $3,
				    updated_by = $4
				WHERE id = $1`,
				equipoID, safeEstado, now, usuarioID,
			); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO equipo_estado_historial
				    (equipo_id, estado_anterior_id, estado_nuevo_id, usuario_id, motivo)
				VALUES ($1, $2, $3, $4, 'Alerta resuelta')`,
				equipoID, estadoActual, safeEstado, usuarioID,
			); err != nil {
				return err
			}
		}

		tag, err := tx.Exec(ctx, `
			UPDATE alerta_evento
			SET resuelta_at = COALESCE(resuelta_at, NOW()),
			    resuelta_por = $2,
			    resolucion_motivo = 'alerta_resuelta_estado_seguro',
			    pospuesta_hasta = NULL,
			    pospuesta_por = NULL
			WHERE equipo_id = $1
			  AND resuelta_at IS NULL`,
			equipoID, usuarioID,
		)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrNotFound
		}
		return nil
	})
}

// Snooze pospone una alerta activa por la duracion indicada.
func (r *AlertaRepo) Snooze(ctx context.Context, alertaID, usuarioID uuid.UUID, d time.Duration) error {
	if d <= 0 {
		d = time.Hour
	}
	tag, err := r.p.Exec(ctx, `
		UPDATE alerta_evento
		SET pospuesta_hasta = NOW() + ($3::text)::interval,
		    pospuesta_por = $2
		WHERE id = $1
		  AND resuelta_at IS NULL`, alertaID, usuarioID, intervalLiteral(d))
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func intervalLiteral(d time.Duration) string {
	minutes := int(d.Minutes())
	if minutes < 1 {
		minutes = 60
	}
	return fmt.Sprintf("%d minutes", minutes)
}

// ListConfig devuelve todos los estados con su configuracion de alertas.
func (r *AlertaRepo) ListConfig(ctx context.Context) ([]domain.AlertaConfig, error) {
	rows, err := r.p.Query(ctx, `
		SELECT
			COALESCE(ac.id, gen_random_uuid()),
			eo.id,
			COALESCE(ac.dias_umbral, 1),
			CASE
			    WHEN lower(eo.nombre) IN ('disponible', 'en uso') THEN FALSE
			    ELSE COALESCE(ac.activa, TRUE)
			END,
			eo.nombre,
			eo.color,
			eo.orden
		FROM estado_operativo eo
		LEFT JOIN alerta_config ac ON ac.estado_id = eo.id
		WHERE eo.activo = TRUE
		ORDER BY eo.orden, eo.nombre`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.AlertaConfig
	for rows.Next() {
		var c domain.AlertaConfig
		if err := rows.Scan(&c.ID, &c.EstadoID, &c.DiasUmbral, &c.Activa, &c.EstadoNombre, &c.EstadoColor, &c.EstadoOrden); err != nil {
			return nil, err
		}
		c.Protegida = estadoSeguro(c.EstadoNombre)
		out = append(out, c)
	}
	return out, rows.Err()
}

// UpsertConfig crea/actualiza la configuracion de un estado.
func (r *AlertaRepo) UpsertConfig(ctx context.Context, estadoID uuid.UUID, dias int, activa bool) (*domain.AlertaConfig, error) {
	var c domain.AlertaConfig
	err := r.p.QueryRow(ctx, `
		WITH estado AS (
			SELECT id, nombre, color, orden
			FROM estado_operativo
			WHERE id = $1 AND activo = TRUE
		), upsert AS (
			INSERT INTO alerta_config (estado_id, dias_umbral, activa)
			SELECT id,
			       GREATEST($2::int, 1),
			       CASE WHEN lower(nombre) IN ('disponible', 'en uso') THEN FALSE ELSE $3 END
			FROM estado
			ON CONFLICT (estado_id) DO UPDATE SET
			    dias_umbral = EXCLUDED.dias_umbral,
			    activa = EXCLUDED.activa
			RETURNING id, estado_id, dias_umbral, activa
		)
		SELECT u.id, u.estado_id, u.dias_umbral, u.activa, e.nombre, e.color, e.orden
		FROM upsert u
		JOIN estado e ON e.id = u.estado_id`,
		estadoID, dias, activa,
	).Scan(&c.ID, &c.EstadoID, &c.DiasUmbral, &c.Activa, &c.EstadoNombre, &c.EstadoColor, &c.EstadoOrden)
	if err != nil {
		return nil, MapPgError(err)
	}
	c.Protegida = estadoSeguro(c.EstadoNombre)
	return &c, nil
}
