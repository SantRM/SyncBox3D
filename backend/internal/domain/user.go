package domain

import (
	"time"

	"github.com/google/uuid"
)

// Role es uno de los tres roles fijos del sistema. La base de datos impone
// esta restricción mediante un CHECK constraint, no mediante una tabla.
type Role string

const (
	RoleAdmin    Role = "ADMINISTRADOR"
	RoleOperator Role = "OPERADOR"
	RoleViewer   Role = "CONSULTA"
)

// Valid indica si el valor coincide con alguno de los tres roles permitidos.
func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleOperator, RoleViewer:
		return true
	}
	return false
}

// Locale representa los idiomas de interfaz soportados por el sistema.
type Locale string

const (
	LocaleES Locale = "es"
	LocaleEN Locale = "en"
)

// Valid indica si el idioma esta soportado por la aplicacion.
func (l Locale) Valid() bool {
	switch l {
	case LocaleES, LocaleEN:
		return true
	}
	return false
}

// Usuario representa una cuenta del sistema (tabla `usuario`).
type Usuario struct {
	ID              uuid.UUID
	Nombre          string
	Correo          string
	PasswordHash    string
	Rol             Role
	Activo          bool
	IdiomaPreferido Locale
	UltimaSesion    *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// PublicUsuario es la vista expuesta por la API (sin hash de contraseña).
type PublicUsuario struct {
	ID              uuid.UUID  `json:"id"`
	Nombre          string     `json:"nombre"`
	Correo          string     `json:"correo"`
	Rol             Role       `json:"rol"`
	Activo          bool       `json:"activo"`
	IdiomaPreferido Locale     `json:"idioma_preferido"`
	UltimaSesion    *time.Time `json:"ultima_sesion,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ToPublic devuelve la vista pública del usuario.
func (u Usuario) ToPublic() PublicUsuario {
	return PublicUsuario{
		ID:              u.ID,
		Nombre:          u.Nombre,
		Correo:          u.Correo,
		Rol:             u.Rol,
		Activo:          u.Activo,
		IdiomaPreferido: u.IdiomaPreferido,
		UltimaSesion:    u.UltimaSesion,
		CreatedAt:       u.CreatedAt,
	}
}
