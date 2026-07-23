package domain

import (
	"time"

	"github.com/google/uuid"
)

// NodoTipo enumera los tipos del árbol de ubicaciones.
type NodoTipo string

const (
	NodoUbicacion   NodoTipo = "UBICACION"
	NodoLaboratorio NodoTipo = "LABORATORIO"
	NodoEquipo      NodoTipo = "EQUIPO"
)

// Valid retorna true si el tipo es uno de los soportados.
func (t NodoTipo) Valid() bool {
	switch t {
	case NodoUbicacion, NodoLaboratorio, NodoEquipo:
		return true
	}
	return false
}

// Nodo representa un nodo del árbol jerárquico (UBICACION/LABORATORIO/EQUIPO).
type Nodo struct {
	ID        uuid.UUID  `json:"id"`
	Tipo      NodoTipo   `json:"tipo"`
	ParentID  *uuid.UUID `json:"parent_id,omitempty"`
	Nombre    string     `json:"nombre"`
	Slug      string     `json:"slug"`
	Orden     int        `json:"orden"`
	Path      string     `json:"path"`
	Depth     int        `json:"depth"`
	Activo    bool       `json:"activo"`
	DeletedAt *time.Time `json:"-"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	CreatedBy *uuid.UUID `json:"created_by,omitempty"`
	UpdatedBy *uuid.UUID `json:"updated_by,omitempty"`
}
