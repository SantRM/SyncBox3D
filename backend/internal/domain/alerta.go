package domain

import (
	"time"

	"github.com/google/uuid"
)

// AlertaConfig define el umbral por estado tras el cual se genera una alerta.
type AlertaConfig struct {
	ID           uuid.UUID `json:"id"`
	EstadoID     uuid.UUID `json:"estado_id"`
	DiasUmbral   int       `json:"dias_umbral"`
	Activa       bool      `json:"activa"`
	EstadoNombre string    `json:"estado_nombre,omitempty"`
	EstadoColor  string    `json:"estado_color,omitempty"`
	EstadoOrden  int       `json:"estado_orden,omitempty"`
	Protegida    bool      `json:"protegida,omitempty"`
}

// AlertaEvento es una alerta concreta sobre un equipo.
type AlertaEvento struct {
	ID               uuid.UUID  `json:"id"`
	EquipoID         uuid.UUID  `json:"equipo_id"`
	EstadoID         uuid.UUID  `json:"estado_id"`
	GeneradaAt       time.Time  `json:"generada_at"`
	VistaAt          *time.Time `json:"vista_at,omitempty"`
	VistaPor         *uuid.UUID `json:"vista_por,omitempty"`
	ResueltaAt       *time.Time `json:"resuelta_at,omitempty"`
	ResueltaPor      *uuid.UUID `json:"resuelta_por,omitempty"`
	ResolucionMotivo string     `json:"resolucion_motivo,omitempty"`
	PospuestaHasta   *time.Time `json:"pospuesta_hasta,omitempty"`
	PospuestaPor     *uuid.UUID `json:"pospuesta_por,omitempty"`
	UpdatedAt        time.Time  `json:"updated_at,omitempty"`

	EquipoNombre string    `json:"equipo_nombre,omitempty"`
	EstadoNombre string    `json:"estado_nombre,omitempty"`
	EstadoColor  string    `json:"estado_color,omitempty"`
	EstadoDesde  time.Time `json:"estado_desde,omitempty"`
	DiasUmbral   int       `json:"dias_umbral,omitempty"`
	DiasEnEstado int       `json:"dias_en_estado,omitempty"`
	Razon        string    `json:"razon,omitempty"`
}
