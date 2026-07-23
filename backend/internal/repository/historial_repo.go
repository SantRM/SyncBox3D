package repository

import (
	"context"

	"github.com/google/uuid"
)

// HistorialRepo registra cambios en `cambio_historial`. Esquema real:
//
//	(id, entidad, entidad_id, usuario_id, campo, valor_anterior, valor_nuevo, fecha)
//
// El registro es a nivel de campo individual; el helper Record acepta una
// lista de mutaciones para insertarlas en la misma llamada.
type HistorialRepo struct{ p *Pool }

// NewHistorialRepo construye el repositorio.
func NewHistorialRepo(p *Pool) *HistorialRepo { return &HistorialRepo{p: p} }

// Mutacion describe un cambio puntual.
type Mutacion struct {
	Campo    string
	Anterior string
	Nuevo    string
}

// Valores permitidos para `entidad` (CHECK en BD).
const (
	EntEquipo          = "EQUIPO"
	EntUsuario         = "USUARIO"
	EntCategoria       = "CATEGORIA"
	EntEstadoOperativo = "ESTADO_OPERATIVO"
	EntAlertaConfig    = "ALERTA_CONFIG"
	EntFichaTecnica    = "FICHA_TECNICA"
)

// Record persiste una o varias mutaciones del mismo registro/entidad.
func (r *HistorialRepo) Record(ctx context.Context, entidad string, entidadID, usuarioID uuid.UUID, muts []Mutacion) error {
	if len(muts) == 0 {
		return nil
	}
	for _, m := range muts {
		_, err := r.p.Exec(ctx, `
			INSERT INTO cambio_historial
			    (entidad, entidad_id, usuario_id, campo, valor_anterior, valor_nuevo)
			VALUES ($1, $2, $3, $4, NULLIF($5,''), NULLIF($6,''))`,
			entidad, entidadID, usuarioID, m.Campo, m.Anterior, m.Nuevo)
		if err != nil {
			return err
		}
	}
	return nil
}
