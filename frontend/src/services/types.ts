// Tipos compartidos espejo del backend (gitlab.com/syncbox/backend).

export type Role = 'ADMINISTRADOR' | 'OPERADOR' | 'CONSULTA'
export type LocaleCode = 'es' | 'en'

export interface PublicUsuario {
  id: string
  nombre: string
  correo: string
  rol: Role
  activo: boolean
  idioma_preferido: LocaleCode
  ultima_sesion?: string | null
  created_at: string
}

export interface AuthResult {
  access_token: string
  refresh_token: string
  access_expires_at: string
  refresh_expires_at: string
  user: PublicUsuario
}

export interface Categoria {
  id: string
  nombre: string
  descripcion: string
  activo: boolean
  created_at: string
  updated_at: string
}

export interface EstadoOperativo {
  id: string
  nombre: string
  color: string
  orden: number
  activo: boolean
}

export interface Equipo {
  id: string
  nombre: string
  fabricante?: string
  modelo?: string
  serial?: string
  ubicacion?: string
  nodo_id?: string | null
  modelo_3d_id?: string | null
  categoria_id: string
  estado_id: string
  estado_desde: string
  activo: boolean
  created_at: string
  updated_at: string
  updated_by?: string | null
}

// --- Árbol jerárquico (UBICACION / LABORATORIO / EQUIPO) ---

export type NodoTipo = 'UBICACION' | 'LABORATORIO' | 'EQUIPO'

export interface Nodo {
  id: string
  tipo: NodoTipo
  parent_id?: string | null
  nombre: string
  slug: string
  orden: number
  path: string
  depth: number
  activo: boolean
  created_at: string
  updated_at: string
}

export interface NodoTreeItem extends Nodo {
  children?: NodoTreeItem[]
  hasChildren?: boolean
  loaded?: boolean
}

// --- Modelos 3D reusables ---

export interface Modelo3D {
  id: string
  nombre: string
  descripcion?: string
  mime: string
  tamano_bytes: number
  sha256: string
  preview_uri?: string
  activo: boolean
  created_at: string
  updated_at: string
}

export interface AlertaEvento {
  id: string
  equipo_id: string
  estado_id: string
  generada_at: string
  vista_at?: string | null
  vista_por?: string | null
  resuelta_at?: string | null
  resuelta_por?: string | null
  resolucion_motivo?: string
  pospuesta_hasta?: string | null
  pospuesta_por?: string | null
  updated_at?: string
  equipo_nombre?: string
  estado_nombre?: string
  estado_color?: string
  estado_desde?: string
  dias_umbral?: number
  dias_en_estado?: number
  razon?: string
}

export interface AlertaConfig {
  id: string
  estado_id: string
  dias_umbral: number
  activa: boolean
  estado_nombre?: string
  estado_color?: string
  estado_orden?: number
  protegida?: boolean
}

export interface FichaTecnica {
  equipo_id: string
  peso?: number | null
  potencia?: number | null
  dimensiones?: string
  anio?: number | null
  observaciones?: string
  atributos_extra: Record<string, unknown>
}

export interface EstadoHistorialEntry {
  id: string
  fecha: string
  estado_anterior_id?: string | null
  estado_anterior?: string
  estado_nuevo_id: string
  estado_nuevo: string
  estado_nuevo_color?: string
  usuario_id: string
  usuario_nombre: string
  motivo?: string
}

export interface CambioEntry {
  id: string
  fecha: string
  campo: string
  valor_anterior?: string
  valor_nuevo?: string
  usuario_id: string
  usuario_nombre: string
}

export interface HistorialResp {
  estados: EstadoHistorialEntry[]
  cambios: CambioEntry[]
}

export interface ApiError {
  status: number
  message: string
  code?: string
}

// --- Nivel 2: Laboratorios (escenas) ---

export interface Escena {
  id: string
  nombre: string
  descripcion: string
  activo: boolean
  nodo_id?: string | null
  iluminacion: EscenaLight
  created_at: string
  updated_at: string
  created_by?: string | null
  updated_by?: string | null
}

export interface EscenaLight {
  activa: boolean
  intensidad: number
  color: string
  pos_x: number
  pos_y: number
  pos_z: number
  target_x: number
  target_y: number
  target_z: number
  angulo: number
  penumbra: number
  distancia: number
  auto_target: boolean
}

export interface LabSesion {
  id: string
  escena_id: string
  usuario_id?: string | null
  iniciada_at: string
  cerrada_at?: string | null
  ultima_actividad_at: string
  cierre_motivo: string
}

export interface LabAuditEntry {
  lab_sesion_id?: string
  instancia_id: string
  escena_id: string
  usuario_id?: string | null
  event_type: 'add' | 'transform' | 'restore' | 'restore_session' | 'remove'
  usuario_nombre: string
  usuario_correo: string
  sesion_iniciada_at: string
  sesion_cerrada_at?: string | null
  sesion_ultima_actividad_at: string
  cierre_motivo: string
  fecha: string
  equipo_origen_id?: string | null
  nombre_snapshot: string
  fabricante_snapshot: string
  modelo_snapshot: string
  categoria_snapshot: string
  pos_x: number
  pos_y: number
  pos_z: number
  escala: number
  rot_x: number
  rot_y: number
  rot_z: number
}

export interface LabAuditResp {
  items: LabAuditEntry[]
  total: number
  limit: number
  offset: number
}

export interface EscenaInstancia {
  id: string
  escena_id: string
  equipo_origen_id?: string | null
  orden: number

  nombre_snapshot: string
  fabricante_snapshot: string
  modelo_snapshot: string
  categoria_snapshot: string

  pos_x: number
  pos_y: number
  pos_z: number
  escala: number
  rot_x: number
  rot_y: number
  rot_z: number

  pos_inicial_x: number
  pos_inicial_y: number
  pos_inicial_z: number
  escala_inicial: number
  rot_inicial_x: number
  rot_inicial_y: number
  rot_inicial_z: number

  created_at: string
  updated_at: string
}

export interface EscenaDetail extends Escena {
  instancias: EscenaInstancia[]
}
