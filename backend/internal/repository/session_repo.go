package repository

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// SessionRepo gestiona la tabla `sesion`, que registra el `jti` de cada token
// emitido para poder revocarlo (sesion: jti, usuario_id, emitido_at, expira_at,
// revocado_at).
type SessionRepo struct{ p *Pool }

// NewSessionRepo construye el repositorio.
func NewSessionRepo(p *Pool) *SessionRepo { return &SessionRepo{p: p} }

// Create persiste un jti emitido. expira_at corresponde al refresh token.
// El parámetro jti debe ser un UUID válido (lo es: lo generamos con uuid.NewString).
func (r *SessionRepo) Create(ctx context.Context, jti string, userID uuid.UUID, expiresAt time.Time) error {
	_, err := r.p.Exec(ctx, `
		INSERT INTO sesion (jti, usuario_id, expira_at)
		VALUES ($1, $2, $3)`,
		jti, userID, expiresAt)
	return err
}

// IsActive informa si el jti existe y no ha sido revocado ni ha expirado.
// Distingue "no encontrado" (false, nil) de un error real de BD (false, err).
func (r *SessionRepo) IsActive(ctx context.Context, jti string) (bool, error) {
	var ok bool
	err := r.p.QueryRow(ctx, `
		SELECT TRUE FROM sesion
		WHERE jti = $1 AND revocado_at IS NULL AND expira_at > NOW()`, jti).Scan(&ok)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return ok, nil
}

// Revoke invalida un jti concreto.
func (r *SessionRepo) Revoke(ctx context.Context, jti string) error {
	_, err := r.p.Exec(ctx, `
		UPDATE sesion SET revocado_at = NOW()
		WHERE jti = $1 AND revocado_at IS NULL`, jti)
	return err
}

// RevokeAllForUser invalida todas las sesiones de un usuario.
func (r *SessionRepo) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.p.Exec(ctx, `
		UPDATE sesion SET revocado_at = NOW()
		WHERE usuario_id = $1 AND revocado_at IS NULL`, userID)
	return err
}
