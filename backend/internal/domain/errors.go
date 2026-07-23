package domain

import "errors"

// Errores de dominio. Los handlers los traducen a códigos HTTP.
var (
	ErrNotFound          = errors.New("recurso no encontrado")
	ErrConflict          = errors.New("conflicto de estado")
	ErrInvalidInput      = errors.New("entrada inválida")
	ErrUnauthorized      = errors.New("no autenticado")
	ErrForbidden         = errors.New("no autorizado")
	ErrInvalidCredential = errors.New("credenciales inválidas")
	ErrAccountBlocked    = errors.New("cuenta bloqueada temporalmente")
	ErrAccountInactive   = errors.New("cuenta inactiva")
	ErrTokenInvalid      = errors.New("token inválido")
	ErrTokenRevoked      = errors.New("token revocado")
	ErrLastAdmin         = errors.New("no se puede eliminar al último administrador activo")
	ErrModelEmpty        = errors.New("archivo de modelo vacio")
	ErrModelTooLarge     = errors.New("el archivo de modelo supera el limite permitido")
	ErrModelFormat       = errors.New("solo se aceptan modelos .glb o .gltf")
	ErrNodoTienHijos     = errors.New("el nodo tiene hijos: se requiere replacement_parent_id o promote=true")
	ErrNodoCiclo         = errors.New("el movimiento crearia un ciclo en el arbol")
	ErrNodoTipoInvalido  = errors.New("tipo de nodo no permitido en este contexto")
	ErrConfirmRequerida  = errors.New("se requiere confirmacion literal 'entiendo'")
	ErrModeloEnUso       = errors.New("modelo 3D referenciado por equipos, no puede borrarse")
)
