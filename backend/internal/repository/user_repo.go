package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gitlab.com/syncbox/backend/internal/domain"
)

// adminInvariantLockKey identifica el advisory lock global que serializa
// cualquier mutación que pueda alterar la cantidad de administradores
// activos. El valor es arbitrario pero estable en todo el cluster.
const adminInvariantLockKey int64 = 0x53594E43_41444D49 // "SYNCADMI"

// UserRepo encapsula las queries sobre la tabla `usuario`.
type UserRepo struct{ p *Pool }

// NewUserRepo crea un repositorio de usuarios.
func NewUserRepo(p *Pool) *UserRepo { return &UserRepo{p: p} }

const userCols = `id, nombre, correo, password_hash, rol, activo, idioma_preferido, ultima_sesion, created_at, updated_at`

func scanUser(row pgx.Row) (*domain.Usuario, error) {
	var u domain.Usuario
	err := row.Scan(&u.ID, &u.Nombre, &u.Correo, &u.PasswordHash,
		&u.Rol, &u.Activo, &u.IdiomaPreferido, &u.UltimaSesion, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &u, nil
}

// FindByEmail busca por correo (case-insensitive).
func (r *UserRepo) FindByEmail(ctx context.Context, email string) (*domain.Usuario, error) {
	row := r.p.QueryRow(ctx, `SELECT `+userCols+` FROM usuario WHERE LOWER(correo) = LOWER($1)`, email)
	return scanUser(row)
}

// FindByID busca por id.
func (r *UserRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Usuario, error) {
	row := r.p.QueryRow(ctx, `SELECT `+userCols+` FROM usuario WHERE id = $1`, id)
	return scanUser(row)
}

// List devuelve todos los usuarios ordenados por fecha de creación.
func (r *UserRepo) List(ctx context.Context) ([]domain.Usuario, error) {
	rows, err := r.p.Query(ctx, `SELECT `+userCols+` FROM usuario ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Usuario
	for rows.Next() {
		var u domain.Usuario
		if err := rows.Scan(&u.ID, &u.Nombre, &u.Correo, &u.PasswordHash,
			&u.Rol, &u.Activo, &u.IdiomaPreferido, &u.UltimaSesion, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// Create inserta un usuario nuevo.
func (r *UserRepo) Create(ctx context.Context, u *domain.Usuario, actor *uuid.UUID) error {
	row := r.p.QueryRow(ctx, `
		INSERT INTO usuario (nombre, correo, password_hash, rol, activo, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, idioma_preferido, created_at, updated_at`,
		u.Nombre, u.Correo, u.PasswordHash, u.Rol, u.Activo, actor)
	return MapPgError(row.Scan(&u.ID, &u.IdiomaPreferido, &u.CreatedAt, &u.UpdatedAt))
}

// Update aplica un parche al usuario. Solo se modifican los campos no nulos.
func (r *UserRepo) Update(ctx context.Context, id uuid.UUID, nombre *string, rol *domain.Role, activo *bool, actor uuid.UUID) error {
	tag, err := r.p.Exec(ctx, `
		UPDATE usuario SET
			nombre   = COALESCE($2, nombre),
			rol      = COALESCE($3, rol),
			activo   = COALESCE($4, activo),
			updated_by = $5
		WHERE id = $1`,
		id, nombre, rol, activo, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UpdatePassword cambia el hash de la contraseña.
func (r *UserRepo) UpdatePassword(ctx context.Context, id uuid.UUID, hash string) error {
	tag, err := r.p.Exec(ctx, `UPDATE usuario SET password_hash = $2 WHERE id = $1`, id, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// UpdatePreferredLanguage cambia la preferencia de idioma del usuario autenticado.
func (r *UserRepo) UpdatePreferredLanguage(ctx context.Context, id uuid.UUID, locale domain.Locale) error {
	tag, err := r.p.Exec(ctx, `UPDATE usuario SET idioma_preferido = $2 WHERE id = $1`, id, locale)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// TouchLastLogin actualiza ultima_sesion.
func (r *UserRepo) TouchLastLogin(ctx context.Context, id uuid.UUID, t time.Time) error {
	_, err := r.p.Exec(ctx, `UPDATE usuario SET ultima_sesion = $2 WHERE id = $1`, id, t)
	return err
}

// CountActiveAdmins cuenta administradores activos.
func (r *UserRepo) CountActiveAdmins(ctx context.Context) (int, error) {
	var n int
	err := r.p.QueryRow(ctx,
		`SELECT COUNT(*) FROM usuario WHERE rol = 'ADMINISTRADOR' AND activo = TRUE`).Scan(&n)
	return n, err
}

// SetActive activa o desactiva un usuario.
func (r *UserRepo) SetActive(ctx context.Context, id uuid.UUID, active bool, actor uuid.UUID) error {
	tag, err := r.p.Exec(ctx,
		`UPDATE usuario SET activo = $2, updated_by = $3 WHERE id = $1`, id, active, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// WithAdminTx ejecuta fn dentro de una transacción serializada por un
// advisory lock que impide condiciones de carrera al validar el invariante
// "al menos un administrador activo". Cualquier ruta que pueda alterar
// dicho conteo (cambio de rol, activación/desactivación) DEBE pasar por
// aquí para garantizar atomicidad lectura→validación→escritura.
func (r *UserRepo) WithAdminTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return r.p.WithTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, adminInvariantLockKey); err != nil {
			return err
		}
		return fn(tx)
	})
}

// FindByIDTx variante de FindByID que opera dentro de una transacción.
func (r *UserRepo) FindByIDTx(ctx context.Context, tx pgx.Tx, id uuid.UUID) (*domain.Usuario, error) {
	row := tx.QueryRow(ctx, `SELECT `+userCols+` FROM usuario WHERE id = $1`, id)
	return scanUser(row)
}

// CountActiveAdminsTx variante transaccional.
func (r *UserRepo) CountActiveAdminsTx(ctx context.Context, tx pgx.Tx) (int, error) {
	var n int
	err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM usuario WHERE rol = 'ADMINISTRADOR' AND activo = TRUE`).Scan(&n)
	return n, err
}

// UpdateTx variante transaccional de Update.
func (r *UserRepo) UpdateTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, nombre *string, rol *domain.Role, activo *bool, actor uuid.UUID) error {
	tag, err := tx.Exec(ctx, `
		UPDATE usuario SET
			nombre   = COALESCE($2, nombre),
			rol      = COALESCE($3, rol),
			activo   = COALESCE($4, activo),
			updated_by = $5
		WHERE id = $1`,
		id, nombre, rol, activo, actor)
	if err != nil {
		return MapPgError(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// SetActiveTx variante transaccional de SetActive.
func (r *UserRepo) SetActiveTx(ctx context.Context, tx pgx.Tx, id uuid.UUID, active bool, actor uuid.UUID) error {
	tag, err := tx.Exec(ctx,
		`UPDATE usuario SET activo = $2, updated_by = $3 WHERE id = $1`, id, active, actor)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
