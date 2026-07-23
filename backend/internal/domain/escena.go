package domain

import (
	"time"

	"github.com/google/uuid"
)

// Escena representa un "laboratorio": un espacio 3D donde se pueden colocar
// múltiples instancias de equipos para visualizarlos y manipularlos.
type Escena struct {
	ID          uuid.UUID   `json:"id"`
	Nombre      string      `json:"nombre"`
	Descripcion string      `json:"descripcion"`
	Activo      bool        `json:"activo"`
	NodoID      *uuid.UUID  `json:"nodo_id,omitempty"`
	Iluminacion EscenaLight `json:"iluminacion"`
	DeletedAt   *time.Time  `json:"-"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	CreatedBy   *uuid.UUID  `json:"created_by,omitempty"`
	UpdatedBy   *uuid.UUID  `json:"updated_by,omitempty"`
}

// EscenaLight configura un foco/lampara propio del laboratorio.
// No afecta a los modelos originales; solo cambia la visualizacion de la escena.
type EscenaLight struct {
	Activa     bool    `json:"activa"`
	Intensidad float64 `json:"intensidad"`
	Color      string  `json:"color"`
	PosX       float64 `json:"pos_x"`
	PosY       float64 `json:"pos_y"`
	PosZ       float64 `json:"pos_z"`
	TargetX    float64 `json:"target_x"`
	TargetY    float64 `json:"target_y"`
	TargetZ    float64 `json:"target_z"`
	Angulo     float64 `json:"angulo"`
	Penumbra   float64 `json:"penumbra"`
	Distancia  float64 `json:"distancia"`
	AutoTarget bool    `json:"auto_target"`
}

// LabSesion registra una entrada al modo interactivo de un laboratorio.
// Los snapshots asociados guardan el ultimo transform de cada instancia durante
// esa sesion, incluso si el navegador se cierra sin avisar.
type LabSesion struct {
	ID                uuid.UUID  `json:"id"`
	EscenaID          uuid.UUID  `json:"escena_id"`
	UsuarioID         *uuid.UUID `json:"usuario_id,omitempty"`
	IniciadaAt        time.Time  `json:"iniciada_at"`
	CerradaAt         *time.Time `json:"cerrada_at,omitempty"`
	UltimaActividadAt time.Time  `json:"ultima_actividad_at"`
	CierreMotivo      string     `json:"cierre_motivo"`
}

// LabSesionInstancia guarda el ultimo transform conocido para una instancia
// dentro de una sesion de laboratorio.
type LabSesionInstancia struct {
	LabSesionID uuid.UUID `json:"lab_sesion_id"`
	InstanciaID uuid.UUID `json:"instancia_id"`
	PosX        float64   `json:"pos_x"`
	PosY        float64   `json:"pos_y"`
	PosZ        float64   `json:"pos_z"`
	Escala      float64   `json:"escala"`
	RotX        float64   `json:"rot_x"`
	RotY        float64   `json:"rot_y"`
	RotZ        float64   `json:"rot_z"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// LabAuditEntry es una fila enriquecida de auditoria append-only de cambios
// sobre modelos dentro de un laboratorio.
type LabAuditEntry struct {
	LabSesionID string     `json:"lab_sesion_id,omitempty"`
	InstanciaID uuid.UUID  `json:"instancia_id"`
	EscenaID    uuid.UUID  `json:"escena_id"`
	UsuarioID   *uuid.UUID `json:"usuario_id,omitempty"`
	EventType   string     `json:"event_type"`

	UsuarioNombre string `json:"usuario_nombre"`
	UsuarioCorreo string `json:"usuario_correo"`

	SesionIniciadaAt      time.Time  `json:"sesion_iniciada_at"`
	SesionCerradaAt       *time.Time `json:"sesion_cerrada_at,omitempty"`
	SesionUltimaActividad time.Time  `json:"sesion_ultima_actividad_at"`
	CierreMotivo          string     `json:"cierre_motivo"`
	Fecha                 time.Time  `json:"fecha"`

	EquipoOrigenID     *uuid.UUID `json:"equipo_origen_id,omitempty"`
	NombreSnapshot     string     `json:"nombre_snapshot"`
	FabricanteSnapshot string     `json:"fabricante_snapshot"`
	ModeloSnapshot     string     `json:"modelo_snapshot"`
	CategoriaSnapshot  string     `json:"categoria_snapshot"`

	PosX   float64 `json:"pos_x"`
	PosY   float64 `json:"pos_y"`
	PosZ   float64 `json:"pos_z"`
	Escala float64 `json:"escala"`
	RotX   float64 `json:"rot_x"`
	RotY   float64 `json:"rot_y"`
	RotZ   float64 `json:"rot_z"`
}

type LabAuditResponse struct {
	Items  []LabAuditEntry `json:"items"`
	Total  int             `json:"total"`
	Limit  int             `json:"limit"`
	Offset int             `json:"offset"`
}

// EscenaInstancia es una "máquina colocada" dentro de una escena. Mantiene el
// vínculo al equipo/modelo original, pero guarda un snapshot textual y un
// transform propio del laboratorio para no mutar el modelo principal.
//
// El transform inicial (pos/rot/escala iniciales) se fija al insertar la
// instancia y nunca se actualiza: lo usa el botón "Restore" del UI para
// devolver el objeto a su posicion, rotacion y escala de alta.
type EscenaInstancia struct {
	ID             uuid.UUID  `json:"id"`
	EscenaID       uuid.UUID  `json:"escena_id"`
	EquipoOrigenID *uuid.UUID `json:"equipo_origen_id,omitempty"`
	Orden          int        `json:"orden"`

	NombreSnapshot     string `json:"nombre_snapshot"`
	FabricanteSnapshot string `json:"fabricante_snapshot"`
	ModeloSnapshot     string `json:"modelo_snapshot"`
	CategoriaSnapshot  string `json:"categoria_snapshot"`

	PosX   float64 `json:"pos_x"`
	PosY   float64 `json:"pos_y"`
	PosZ   float64 `json:"pos_z"`
	Escala float64 `json:"escala"`
	RotX   float64 `json:"rot_x"`
	RotY   float64 `json:"rot_y"`
	RotZ   float64 `json:"rot_z"`

	PosInicialX   float64 `json:"pos_inicial_x"`
	PosInicialY   float64 `json:"pos_inicial_y"`
	PosInicialZ   float64 `json:"pos_inicial_z"`
	EscalaInicial float64 `json:"escala_inicial"`
	RotInicialX   float64 `json:"rot_inicial_x"`
	RotInicialY   float64 `json:"rot_inicial_y"`
	RotInicialZ   float64 `json:"rot_inicial_z"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EscenaDetail es la respuesta de GET /escenas/:id (escena + instancias).
type EscenaDetail struct {
	Escena
	Instancias []EscenaInstancia `json:"instancias"`
}
