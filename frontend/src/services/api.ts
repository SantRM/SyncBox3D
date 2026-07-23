// Cliente HTTP centralizado.
//
// Política de tokens:
//  - access_token vive SOLO en memoria (auth store). Se pierde al recargar:
//    correcto, mitiga XSS persistente.
//  - refresh_token se guarda en sessionStorage (no localStorage). El backend
//    lo emite en el body JSON, no como cookie HttpOnly; sessionStorage es el
//    compromiso razonable: muere al cerrar pestaña/navegador.
//  - Ante 401 con token vencido, intenta /auth/refresh UNA vez y reintenta.
//  - Refresh fallido → limpia estado y redirige a /login.

import type { ApiError, AuthResult } from './types'

const BASE_URL = (import.meta.env.VITE_API_BASE_URL ?? '/api/v1').replace(/\/+$/, '')
const REFRESH_KEY = 'syncbox.refresh'

let accessToken: string | null = null
let refreshInFlight: Promise<string | null> | null = null
let onAuthFailure: (() => void) | null = null

export function setAccessToken(t: string | null): void {
  accessToken = t
}

export function getAccessToken(): string | null {
  return accessToken
}

export function setRefreshToken(t: string | null): void {
  if (t) sessionStorage.setItem(REFRESH_KEY, t)
  else sessionStorage.removeItem(REFRESH_KEY)
}

export function getRefreshToken(): string | null {
  return sessionStorage.getItem(REFRESH_KEY)
}

export function setOnAuthFailure(cb: () => void): void {
  onAuthFailure = cb
}

export function clearTokens(): void {
  accessToken = null
  setRefreshToken(null)
}

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'
  body?: unknown
  query?: Record<string, string | number | boolean | undefined | null>
  // Si true, no intenta refresh ni adjunta Authorization (para login/refresh).
  anonymous?: boolean
}

export function buildApiUrl(path: string, query?: RequestOptions['query']): string {
  const url = new URL(BASE_URL + path, window.location.origin)
  if (query) {
    for (const [k, v] of Object.entries(query)) {
      if (v === undefined || v === null || v === '') continue
      url.searchParams.set(k, String(v))
    }
  }
  return url.toString()
}

async function parseError(res: Response): Promise<ApiError> {
  const status = res.status
  let message = res.statusText || 'Error'
  let code: string | undefined
  try {
    const data = await res.json()
    if (typeof data?.message === 'string') message = data.message
    else if (typeof data?.error === 'string') message = data.error
    if (typeof data?.code === 'string') code = data.code
  } catch {
    // body no es JSON; usamos statusText
  }
  return { status, message, code }
}

async function refreshOnce(): Promise<string | null> {
  if (refreshInFlight) return refreshInFlight
  const rt = getRefreshToken()
  if (!rt) return null

  refreshInFlight = (async () => {
    try {
      const res = await fetch(buildApiUrl('/auth/refresh'), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ refresh_token: rt }),
      })
      if (!res.ok) {
        clearTokens()
        return null
      }
      const data = (await res.json()) as AuthResult
      setAccessToken(data.access_token)
      setRefreshToken(data.refresh_token)
      return data.access_token
    } catch {
      clearTokens()
      return null
    } finally {
      refreshInFlight = null
    }
  })()

  return refreshInFlight
}

export async function apiFetch(
  path: string,
  init: RequestInit = {},
  opts: { anonymous?: boolean; query?: RequestOptions['query'] } = {},
): Promise<Response> {
  const headers = new Headers(init.headers)
  if (!opts.anonymous && accessToken) headers.set('Authorization', `Bearer ${accessToken}`)

  const nextInit: RequestInit = {
    ...init,
    headers,
    credentials: 'omit',
  }

  let res = await fetch(buildApiUrl(path, opts.query), nextInit)
  if (res.status === 401 && !opts.anonymous) {
    const fresh = await refreshOnce()
    if (fresh) {
      headers.set('Authorization', `Bearer ${fresh}`)
      res = await fetch(buildApiUrl(path, opts.query), { ...nextInit, headers })
    } else {
      onAuthFailure?.()
    }
  }
  return res
}

export async function request<T = unknown>(path: string, opts: RequestOptions = {}): Promise<T> {
  const headers = new Headers()
  if (opts.body !== undefined) headers.set('Content-Type', 'application/json')

  const res = await apiFetch(path, {
    method: opts.method ?? 'GET',
    headers,
    body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined,
  }, { anonymous: opts.anonymous, query: opts.query })

  if (!res.ok) throw await parseError(res)
  if (res.status === 204) return undefined as T
  const ct = res.headers.get('content-type') ?? ''
  if (ct.includes('application/json')) return (await res.json()) as T
  return (await res.text()) as unknown as T
}

// --- Endpoints tipados ---

import type {
  AlertaConfig,
  AlertaEvento,
  Categoria,
  Equipo,
  Escena,
  EscenaDetail,
  EscenaInstancia,
  EscenaLight,
  EstadoOperativo,
  FichaTecnica,
  HistorialResp,
  LabAuditResp,
  LabSesion,
  LocaleCode,
  Modelo3D,
  Nodo,
  NodoTipo,
  PublicUsuario,
} from './types'

export const api = {
  auth: {
    login: (correo: string, password: string) =>
      request<AuthResult>('/auth/login', {
        method: 'POST',
        body: { correo, password },
        anonymous: true,
      }),
    logout: () => request<void>('/auth/logout', { method: 'POST' }),
    me: () => request<PublicUsuario>('/me'),
    updatePreferences: (idioma: LocaleCode) =>
      request<PublicUsuario>('/me/preferencias', { method: 'PATCH', body: { idioma } }),
    changePassword: (old_password: string, new_password: string) =>
      request<void>('/auth/password', {
        method: 'POST',
        body: { old_password, new_password },
      }),
  },
  categorias: {
    list: (soloActivas = false) =>
      request<Categoria[]>('/categorias', { query: { activas: soloActivas || undefined } }),
    create: (nombre: string, descripcion: string) =>
      request<Categoria>('/categorias', {
        method: 'POST',
        body: { nombre, descripcion, activo: true },
      }),
    update: (
      id: string,
      patch: Partial<Pick<Categoria, 'nombre' | 'descripcion' | 'activo'>>,
    ) => request<void>(`/categorias/${id}`, { method: 'PATCH', body: patch }),
  },
  estados: {
    list: () => request<EstadoOperativo[]>('/estados'),
  },
  equipos: {
    list: (params: {
      q?: string
      categoria_id?: string
      estado_id?: string
      nodo_id?: string
      limit?: number
      offset?: number
    } = {}) => request<Equipo[]>('/equipos', { query: params }),
    get: (id: string) => request<Equipo>(`/equipos/${id}`),
    create: (data: {
      nombre: string
      categoria_id: string
      estado_id: string
      fabricante?: string
      modelo?: string
      serial?: string
      ubicacion?: string
      parent_nodo_id?: string | null
      nodo_id?: string | null
      modelo_3d_id?: string | null
    }) => request<Equipo>('/equipos', { method: 'POST', body: data }),
    update: (id: string, patch: Partial<Equipo>) =>
      request<Equipo>(`/equipos/${id}`, { method: 'PATCH', body: patch }),
    setModelo3D: (id: string, modelo_3d_id: string | null) =>
      request<Equipo>(`/equipos/${id}/modelo3d`, { method: 'PATCH', body: { modelo_3d_id } }),
    delete: (id: string) => request<void>(`/equipos/${id}`, { method: 'DELETE' }),
    changeState: (id: string, estado_id: string, motivo: string) =>
      request<void>(`/equipos/${id}/estado`, {
        method: 'PATCH',
        body: { estado_id, motivo },
      }),
    historial: (id: string) => request<HistorialResp>(`/equipos/${id}/historial`),
    getFicha: (id: string) => request<FichaTecnica | null>(`/equipos/${id}/ficha`),
    upsertFicha: (id: string, data: Partial<FichaTecnica>) =>
      request<FichaTecnica>(`/equipos/${id}/ficha`, { method: 'PUT', body: data }),
  },
  usuarios: {
    list: () => request<PublicUsuario[]>('/usuarios/'),
    get: (id: string) => request<PublicUsuario>(`/usuarios/${id}`),
    create: (data: { nombre: string; correo: string; password: string; rol: string }) =>
      request<PublicUsuario>('/usuarios/', { method: 'POST', body: data }),
    update: (
      id: string,
      patch: { nombre?: string; rol?: string; activo?: boolean },
    ) => request<PublicUsuario>(`/usuarios/${id}`, { method: 'PATCH', body: patch }),
    deactivate: (id: string) =>
      request<void>(`/usuarios/${id}`, { method: 'DELETE' }),
  },
  alertas: {
    list: (params: {
      estado?: 'pendiente' | 'resuelta' | ''
      q?: string
      limit?: number
      offset?: number
    } = {}) => request<AlertaEvento[]>('/alertas/', { query: params }),
    pendientes: (due = false) => request<AlertaEvento[]>('/alertas/pendientes', { query: { due: due || undefined } }),
    resolver: (id: string) =>
      request<void>(`/alertas/${id}/resolver`, { method: 'POST' }),
    posponer: (id: string, minutes = 60) =>
      request<void>(`/alertas/${id}/posponer`, { method: 'POST', body: { minutes } }),
    marcarVista: (id: string) =>
      request<void>(`/alertas/${id}/visto`, { method: 'POST' }),
    config: () => request<AlertaConfig[]>('/alertas/config'),
    updateConfig: (estado_id: string, data: { dias_umbral: number; activa: boolean }) =>
      request<AlertaConfig>(`/alertas/config/${estado_id}`, { method: 'PATCH', body: data }),
  },
  escenas: {
    list: (soloActivas = false) =>
      request<Escena[]>('/escenas/', { query: { activas: soloActivas || undefined } }),
    get: (id: string) => request<EscenaDetail>(`/escenas/${id}`),
    create: (data: { nombre: string; descripcion?: string; nodo_id?: string | null }) =>
      request<Escena>('/escenas/', { method: 'POST', body: data }),
    update: (
      id: string,
      patch: Partial<Pick<Escena, 'nombre' | 'descripcion' | 'activo' | 'nodo_id'>>,
    ) => request<void>(`/escenas/${id}`, { method: 'PATCH', body: patch }),
    updateLighting: (id: string, data: EscenaLight) =>
      request<EscenaLight>(`/escenas/${id}/iluminacion`, { method: 'PATCH', body: data }),
    auditoria: (id: string, params: {
      q?: string
      desde?: string
      hasta?: string
      estado?: string
      limit?: number
      offset?: number
    } = {}) => request<LabAuditResp>(`/escenas/${id}/auditoria`, { query: params }),
    startSesion: (id: string) =>
      request<LabSesion>(`/escenas/${id}/sesiones`, { method: 'POST' }),
    closeSesion: (id: string, sesionId: string, motivo = 'manual') =>
      request<LabSesion>(`/escenas/${id}/sesiones/${sesionId}/cerrar`, {
        method: 'POST',
        body: { motivo },
      }),
    delete: (id: string) => request<void>(`/escenas/${id}`, { method: 'DELETE' }),
    addInstancia: (
      escenaId: string,
      data: {
        equipo_id: string
        lab_sesion_id?: string
        pos_x?: number
        pos_y?: number
        pos_z?: number
        escala?: number
        rot_x?: number
        rot_y?: number
        rot_z?: number
      },
    ) =>
      request<EscenaInstancia>(`/escenas/${escenaId}/instancias`, {
        method: 'POST',
        body: { pos_x: 0, pos_y: 0, pos_z: 0, escala: 1, rot_x: 0, rot_y: 0, rot_z: 0, ...data },
      }),
    updateInstancia: (
      escenaId: string,
      instId: string,
      patch: Partial<Pick<EscenaInstancia, 'pos_x' | 'pos_y' | 'pos_z' | 'escala' | 'rot_x' | 'rot_y' | 'rot_z'>> & { lab_sesion_id?: string },
    ) =>
      request<EscenaInstancia>(`/escenas/${escenaId}/instancias/${instId}`, {
        method: 'PATCH',
        body: patch,
      }),
    restoreInstancia: (escenaId: string, instId: string, labSesionId?: string | null) =>
      request<EscenaInstancia>(`/escenas/${escenaId}/instancias/${instId}/restore`, {
        method: 'POST',
        body: labSesionId ? { lab_sesion_id: labSesionId } : undefined,
      }),
    restoreInstanciaSesion: (escenaId: string, instId: string, labSesionId?: string | null) =>
      request<EscenaInstancia>(`/escenas/${escenaId}/instancias/${instId}/restore-session`, {
        method: 'POST',
        body: labSesionId ? { lab_sesion_id: labSesionId } : undefined,
      }),
    removeInstancia: (escenaId: string, instId: string) =>
      request<void>(`/escenas/${escenaId}/instancias/${instId}`, { method: 'DELETE' }),
  },
  nodos: {
    list: (parent_id?: string) =>
      request<Nodo[]>('/nodos/', { query: parent_id ? { parent_id } : undefined }),
    get: (id: string) => request<Nodo>(`/nodos/${id}`),
    children: (id: string) => request<Nodo[]>(`/nodos/${id}/children`),
    subtree: (id: string) => request<Nodo[]>(`/nodos/${id}/subtree`),
    ancestors: (id: string) => request<Nodo[]>(`/nodos/${id}/ancestors`),
    create: (data: { tipo: NodoTipo; parent_id?: string | null; nombre: string; slug?: string; orden?: number }) =>
      request<Nodo>('/nodos/', { method: 'POST', body: data }),
    update: (id: string, patch: { nombre?: string; slug?: string; orden?: number }) =>
      request<Nodo>(`/nodos/${id}`, { method: 'PATCH', body: patch }),
    move: (id: string, new_parent_id: string | null) =>
      request<void>(`/nodos/${id}/move`, { method: 'POST', body: { new_parent_id } }),
    delete: (
      id: string,
      opts: { confirm?: string; replacement_parent_id?: string | null; promote?: boolean } = {},
    ) => request<void>(`/nodos/${id}`, { method: 'DELETE', body: opts }),
  },
  modelos3d: {
    list: (q?: string) => request<Modelo3D[]>('/modelos3d/', { query: q ? { q } : undefined }),
    get: (id: string) => request<Modelo3D>(`/modelos3d/${id}`),
    fileUrl: (id: string) => buildApiUrl(`/modelos3d/${id}/file`),
    upload: async (file: File, nombre?: string, descripcion?: string, assets: File[] = []): Promise<Modelo3D> => {
      const fd = new FormData()
      fd.append('file', file, uploadFileName(file))
      fd.append('file_path', uploadFileName(file))
      for (const asset of assets) {
        fd.append('assets', asset, uploadFileName(asset))
        fd.append('asset_path', uploadFileName(asset))
      }
      if (nombre) fd.append('nombre', nombre)
      if (descripcion) fd.append('descripcion', descripcion)
      const res = await apiFetch('/modelos3d/', { method: 'POST', body: fd })
      if (!res.ok) throw await parseErrorPublic(res)
      return (await res.json()) as Modelo3D
    },
    update: (id: string, patch: { nombre?: string; descripcion?: string }) =>
      request<Modelo3D>(`/modelos3d/${id}`, { method: 'PATCH', body: patch }),
    delete: (id: string) => request<void>(`/modelos3d/${id}`, { method: 'DELETE' }),
  },
}

// Reexposición de parseError para uso en api.modelos3d.upload (multipart).
function uploadFileName(file: File): string {
  const rel = (file as File & { webkitRelativePath?: string }).webkitRelativePath
  return rel && rel.trim() ? rel : file.name
}

async function parseErrorPublic(res: Response): Promise<{ status: number; message: string; code?: string }> {
  const status = res.status
  let message = res.statusText || 'Error'
  let code: string | undefined
  try {
    const data = await res.json()
    if (typeof data?.message === 'string') message = data.message
    else if (typeof data?.error === 'string') message = data.error
    if (typeof data?.code === 'string') code = data.code
  } catch { /* noop */ }
  return { status, message, code }
}
