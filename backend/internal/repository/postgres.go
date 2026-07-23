package repository

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"gitlab.com/syncbox/backend/internal/domain"
)

// Pool envuelve a pgxpool.Pool con utilidades de conveniencia.
type Pool struct {
	*pgxpool.Pool
}

// Connect abre el pool con timeouts y límites razonables.
func Connect(ctx context.Context, dsn string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.MaxConns = 20
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 15 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	p, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := p.Ping(pingCtx); err != nil {
		p.Close()
		return nil, err
	}
	return &Pool{Pool: p}, nil
}

// WithTx ejecuta fn dentro de una transacción y la commitea/rollbackea.
func (p *Pool) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	return tx.Commit(ctx)
}

// MapPgError traduce errores típicos de Postgres a errores de dominio.
// 23505 = unique_violation; 23503 = foreign_key_violation.
func MapPgError(err error) error {
	if err == nil {
		return nil
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23505":
			return domain.ErrConflict
		case "23503", "23502", "23514":
			return domain.ErrInvalidInput
		}
	}
	return err
}
