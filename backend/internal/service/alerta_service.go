package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"gitlab.com/syncbox/backend/internal/domain"
	"gitlab.com/syncbox/backend/internal/repository"
)

// AlertaService es el caso de uso de alertas operativas.
type AlertaService struct {
	alertas *repository.AlertaRepo
}

// NewAlertaService construye el servicio.
func NewAlertaService(a *repository.AlertaRepo) *AlertaService { return &AlertaService{alertas: a} }

// Pendientes lista alertas activas. dueOnly limita a las que deben mostrarse en popup.
func (s *AlertaService) Pendientes(ctx context.Context, dueOnly bool) ([]domain.AlertaEvento, error) {
	if _, err := s.alertas.ResolveObsoleteAuto(ctx); err != nil {
		return nil, err
	}
	return s.alertas.ListPendientes(ctx, dueOnly)
}

// ListEventos lista pendientes/resueltas para la vista general.
func (s *AlertaService) ListEventos(ctx context.Context, estado, search string, limit, offset int) ([]domain.AlertaEvento, error) {
	if _, err := s.alertas.ResolveObsoleteAuto(ctx); err != nil {
		return nil, err
	}
	estado = strings.TrimSpace(strings.ToLower(estado))
	if estado != "pendiente" && estado != "resuelta" {
		estado = ""
	}
	return s.alertas.ListEventos(ctx, repository.AlertaListFilters{
		Estado: estado,
		Search: strings.TrimSpace(search),
		Limit:  limit,
		Offset: offset,
	})
}

// MarcarVista conserva compatibilidad con la accion anterior.
func (s *AlertaService) MarcarVista(ctx context.Context, alertaID, actor uuid.UUID) error {
	return s.alertas.MarkVista(ctx, alertaID, actor)
}

// Resolver atiende una alerta y mueve el equipo a un estado seguro para que no
// vuelva a dispararse por la misma condicion.
func (s *AlertaService) Resolver(ctx context.Context, alertaID, actor uuid.UUID) error {
	return s.alertas.ResolveWithSafeState(ctx, alertaID, actor)
}

// Posponer difiere el popup de una alerta activa. Por defecto se repite en 1 hora.
func (s *AlertaService) Posponer(ctx context.Context, alertaID, actor uuid.UUID, minutes int) error {
	if minutes <= 0 {
		minutes = 60
	}
	if minutes > 24*60 {
		minutes = 24 * 60
	}
	return s.alertas.Snooze(ctx, alertaID, actor, time.Duration(minutes)*time.Minute)
}

// Configuracion lista umbrales por estado.
func (s *AlertaService) Configuracion(ctx context.Context) ([]domain.AlertaConfig, error) {
	return s.alertas.ListConfig(ctx)
}

// ActualizarConfig modifica un umbral. El repo protege Disponible/En uso.
func (s *AlertaService) ActualizarConfig(ctx context.Context, estadoID uuid.UUID, dias int, activa bool) (*domain.AlertaConfig, error) {
	if dias <= 0 {
		return nil, domain.ErrInvalidInput
	}
	cfg, err := s.alertas.UpsertConfig(ctx, estadoID, dias, activa)
	if err != nil {
		return nil, err
	}
	_, err = s.alertas.ResolveObsoleteAuto(ctx)
	return cfg, err
}

// ResolverEquipoPorCambioEstado cierra alertas activas de un equipo cuando su
// estado cambia; no hace falta que el usuario pulse "resuelto".
func (s *AlertaService) ResolverEquipoPorCambioEstado(ctx context.Context, equipoID, actor uuid.UUID) error {
	return s.alertas.ResolveActiveForEquipo(ctx, equipoID, actor, "cambio_estado_equipo")
}

// GenerarPendientes recorre los equipos cuyo tiempo en estado supera el umbral
// activo y crea alertas nuevas. Devuelve cuantas se crearon.
func (s *AlertaService) GenerarPendientes(ctx context.Context) (int, error) {
	if _, err := s.alertas.ResolveObsoleteAuto(ctx); err != nil {
		return 0, err
	}
	cands, err := s.alertas.FindCandidatos(ctx)
	if err != nil {
		return 0, err
	}
	creadas := 0
	for _, c := range cands {
		ok, err := s.alertas.CreateEvento(ctx, c.EquipoID, c.EstadoID)
		if err != nil {
			return creadas, err
		}
		if ok {
			creadas++
		}
	}
	return creadas, nil
}
