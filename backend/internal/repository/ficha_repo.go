package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"gitlab.com/syncbox/backend/internal/domain"
)

// FichaTecnica refleja la fila correspondiente a un equipo en `ficha_tecnica`.
type FichaTecnica struct {
	EquipoID       uuid.UUID       `json:"equipo_id"`
	Peso           *float64        `json:"peso,omitempty"`
	Potencia       *float64        `json:"potencia,omitempty"`
	Dimensiones    string          `json:"dimensiones,omitempty"`
	Anio           *int            `json:"anio,omitempty"`
	Observaciones  string          `json:"observaciones,omitempty"`
	AtributosExtra json.RawMessage `json:"atributos_extra"`
}

// FichaRepo accede a la tabla `ficha_tecnica`.
type FichaRepo struct{ p *Pool }

// NewFichaRepo construye el repositorio.
func NewFichaRepo(p *Pool) *FichaRepo { return &FichaRepo{p: p} }

// Get devuelve la ficha; si no existe retorna domain.ErrNotFound.
func (r *FichaRepo) Get(ctx context.Context, equipoID uuid.UUID) (*FichaTecnica, error) {
	var f FichaTecnica
	err := r.p.QueryRow(ctx, `
		SELECT equipo_id, peso, potencia, COALESCE(dimensiones,''), anio,
		       COALESCE(observaciones,''), atributos_extra
		FROM ficha_tecnica WHERE equipo_id = $1`, equipoID,
	).Scan(&f.EquipoID, &f.Peso, &f.Potencia, &f.Dimensiones, &f.Anio, &f.Observaciones, &f.AtributosExtra)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	return &f, nil
}

// Upsert crea o actualiza la ficha de un equipo.
func (r *FichaRepo) Upsert(ctx context.Context, f *FichaTecnica) error {
	if len(f.AtributosExtra) == 0 {
		f.AtributosExtra = json.RawMessage(`{}`)
	}
	_, err := r.p.Exec(ctx, `
		INSERT INTO ficha_tecnica
		    (equipo_id, peso, potencia, dimensiones, anio, observaciones, atributos_extra)
		VALUES ($1, $2, $3, NULLIF($4,''), $5, NULLIF($6,''), $7)
		ON CONFLICT (equipo_id) DO UPDATE SET
		    peso = EXCLUDED.peso,
		    potencia = EXCLUDED.potencia,
		    dimensiones = EXCLUDED.dimensiones,
		    anio = EXCLUDED.anio,
		    observaciones = EXCLUDED.observaciones,
		    atributos_extra = EXCLUDED.atributos_extra`,
		f.EquipoID, f.Peso, f.Potencia, f.Dimensiones, f.Anio, f.Observaciones, f.AtributosExtra)
	return MapPgError(err)
}
