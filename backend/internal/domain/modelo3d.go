package domain

import (
	"time"

	"github.com/google/uuid"
)

// Modelo3D es un archivo 3D reusable. La unicidad por sha256 permite dedup.
type Modelo3D struct {
	ID          uuid.UUID  `json:"id"`
	Nombre      string     `json:"nombre"`
	Descripcion string     `json:"descripcion,omitempty"`
	Mime        string     `json:"mime"`
	TamanoBytes int64      `json:"tamano_bytes"`
	SHA256      string     `json:"sha256"`
	StorageURI  string     `json:"-"`
	PreviewURI  string     `json:"preview_uri,omitempty"`
	Activo      bool       `json:"activo"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	CreatedBy   *uuid.UUID `json:"created_by,omitempty"`
	UpdatedBy   *uuid.UUID `json:"updated_by,omitempty"`
}
