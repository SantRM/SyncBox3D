package repository

import (
	"context"
	"time"
)

// LoginAttemptRepo maneja la tabla `intento_login` para anti–brute-force.
// Schema real: id, correo, ip, exito, fecha (sin user_agent).
type LoginAttemptRepo struct{ p *Pool }

// NewLoginAttemptRepo construye el repositorio.
func NewLoginAttemptRepo(p *Pool) *LoginAttemptRepo { return &LoginAttemptRepo{p: p} }

// Record persiste un intento (exitoso o no).
func (r *LoginAttemptRepo) Record(ctx context.Context, correo, ip string, exito bool) error {
	_, err := r.p.Exec(ctx, `
		INSERT INTO intento_login (correo, ip, exito)
		VALUES ($1, NULLIF($2,''), $3)`,
		correo, ip, exito)
	return err
}

// CountFailsByIP cuenta los intentos fallidos desde una IP en la ventana,
// independientemente del correo. Sirve para detectar enumeración a gran
// escala (un atacante probando muchos correos desde la misma IP).
// Devuelve 0 si la IP es vacía (no se puede atribuir).
func (r *LoginAttemptRepo) CountFailsByIP(ctx context.Context, ip string, since time.Time) (int, error) {
	if ip == "" {
		return 0, nil
	}
	var n int
	err := r.p.QueryRow(ctx, `
		SELECT COUNT(*) FROM intento_login
		WHERE ip = $1 AND exito = FALSE AND fecha >= $2`,
		ip, since).Scan(&n)
	return n, err
}

// CountFails devuelve los fallos para un correo dentro de la ventana indicada,
// contando únicamente los posteriores al último éxito (si existe). De esta
// forma un login exitoso resetea efectivamente el contador sin perder el
// rastro auditable de los intentos.
func (r *LoginAttemptRepo) CountFails(ctx context.Context, correo string, since time.Time) (int, error) {
	var n int
	err := r.p.QueryRow(ctx, `
		SELECT COUNT(*) FROM intento_login
		WHERE LOWER(correo) = LOWER($1)
		  AND exito = FALSE
		  AND fecha >= $2
		  AND fecha > COALESCE(
		      (SELECT MAX(fecha) FROM intento_login
		       WHERE LOWER(correo) = LOWER($1) AND exito = TRUE),
		      'epoch'::timestamptz
		  )`,
		correo, since).Scan(&n)
	return n, err
}
