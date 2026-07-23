package domain

import (
	"time"

	"github.com/google/uuid"
)

// Categoria agrupa equipos por familia funcional (Soldadura, Corte, etc.).
type Categoria struct {
	ID          uuid.UUID `json:"id"`
	Nombre      string    `json:"nombre"`
	Descripcion string    `json:"descripcion"`
	Activo      bool      `json:"activo"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// EstadoOperativo es el estado actual de un equipo.
type EstadoOperativo struct {
	ID     uuid.UUID `json:"id"`
	Nombre string    `json:"nombre"`
	Color  string    `json:"color"`
	Orden  int       `json:"orden"`
	Activo bool      `json:"activo"`
}

// Equipo representa una máquina o herramienta del catálogo de Syncbox.
type Equipo struct {
	ID          uuid.UUID  `json:"id"`
	Nombre      string     `json:"nombre"`
	Fabricante  string     `json:"fabricante,omitempty"`
	Modelo      string     `json:"modelo,omitempty"`
	Serial      string     `json:"serial,omitempty"`
	Ubicacion   string     `json:"ubicacion,omitempty"` // legacy free-text; se mantiene por compatibilidad.
	NodoID      *uuid.UUID `json:"nodo_id,omitempty"`
	Modelo3DID  *uuid.UUID `json:"modelo_3d_id,omitempty"`
	CategoriaID uuid.UUID  `json:"categoria_id"`
	EstadoID    uuid.UUID  `json:"estado_id"`
	EstadoDesde time.Time  `json:"estado_desde"`
	Activo      bool       `json:"activo"`
	DeletedAt   *time.Time `json:"-"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	UpdatedBy   *uuid.UUID `json:"updated_by,omitempty"`
}
