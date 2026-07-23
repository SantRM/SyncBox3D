// Cliente de modelos 3D para equipos.
// La ruta canonica es: subir al catalogo /modelos3d y enlazar el equipo por
// modelo_3d_id. El archivo fisico se sirve siempre desde /modelos3d/:id/file.

import { apiFetch, buildApiUrl } from './api'
import type { Equipo, Modelo3D } from './types'

const SKETCHFAB_KEY = (id: string) => `syncbox.sketchfab.${id}`

async function apiError(res: Response, fallback: string): Promise<Error> {
  try {
    const data = await res.json()
    if (typeof data?.message === 'string') return new Error(data.message)
    if (typeof data?.error === 'string') return new Error(data.error)
  } catch {
    // body no JSON
  }
  return new Error(fallback)
}

export function modelName(file: File): string {
  return file.name.replace(/\.(glb|gltf)$/i, '').trim() || file.name
}

export function isAcceptedModelFile(file: File): boolean {
  return /\.(glb|gltf)$/i.test(file.name)
}

export function pickMainGltf(files: File[]): File | null {
  const gltfs = files
    .filter((file) => /\.gltf$/i.test(file.name))
    .sort((a, b) => {
      const ar = relativeDepth(a)
      const br = relativeDepth(b)
      if (ar !== br) return ar - br
      return uploadRelativePath(a).localeCompare(uploadRelativePath(b))
    })
  return gltfs[0] ?? null
}

export function uploadRelativePath(file: File): string {
  const rel = (file as File & { webkitRelativePath?: string }).webkitRelativePath
  return rel && rel.trim() ? rel : file.name
}

export function totalModelBytes(files: File[]): number {
  return files.reduce((sum, file) => sum + file.size, 0)
}

function relativeDepth(file: File): number {
  return uploadRelativePath(file).split('/').length
}

async function setEquipoModelo(equipoId: string, modelo3dId: string | null): Promise<Equipo> {
  const res = await apiFetch(`/equipos/${equipoId}/modelo3d`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ modelo_3d_id: modelo3dId }),
  })
  if (!res.ok) throw await apiError(res, 'No se pudo actualizar el modelo del equipo.')
  return (await res.json()) as Equipo
}

export const modelStore = {
  async save(equipoId: string, file: File): Promise<Modelo3D> {
    const form = new FormData()
    form.append('file', file, uploadRelativePath(file))
    form.append('file_path', uploadRelativePath(file))
    form.append('nombre', modelName(file))

    const upload = await apiFetch('/modelos3d/', { method: 'POST', body: form })
    if (!upload.ok) throw await apiError(upload, 'No se pudo subir el modelo.')

    const modelo = (await upload.json()) as Modelo3D
    await setEquipoModelo(equipoId, modelo.id)
    this.setSketchfabId(equipoId, null)
    return modelo
  },

  async getObjectUrl(equipoId: string): Promise<string | null> {
    const res = await apiFetch(`/equipos/${equipoId}`)
    if (res.status === 404) return null
    if (!res.ok) throw await apiError(res, 'No se pudo consultar el equipo.')

    const equipo = (await res.json()) as Equipo
    return equipo.modelo_3d_id ? buildApiUrl(`/modelos3d/${equipo.modelo_3d_id}/file`) : null
  },

  async remove(equipoId: string): Promise<void> {
    await setEquipoModelo(equipoId, null)
    this.setSketchfabId(equipoId, null)
  },

  getSketchfabId(equipoId: string): string | null {
    return localStorage.getItem(SKETCHFAB_KEY(equipoId))
  },

  setSketchfabId(equipoId: string, id: string | null): void {
    if (id && id.trim()) localStorage.setItem(SKETCHFAB_KEY(equipoId), id.trim())
    else localStorage.removeItem(SKETCHFAB_KEY(equipoId))
  },
}

// Tamano maximo aceptado por la UI. Debe mantenerse alineado con MODEL_MAX_MB
// del backend para que el rechazo sea claro antes de iniciar la subida.
export const MAX_MODEL_BYTES = 500 * 1024 * 1024 // 500 MB
