<script setup lang="ts">
// Vista de edición de un laboratorio (escena).
//
// Layout:
//   ┌─────────────┬──────────────────────────────┐
//   │ Sidebar     │  LabViewer3D (canvas)        │
//   │ (catálogo)  │                              │
//   │             ├──────────────────────────────┤
//   │             │ Panel de instancia           │
//   └─────────────┴──────────────────────────────┘
//
// Roles:
//  - Administrador y Operador → mutaciones (añadir, mover, escalar, eliminar).
//  - Consulta → solo visualiza; los controles destructivos se ocultan.
//
// Los .glb se resuelven vía modelStore desde el almacenamiento interno del
// backend. Si un equipo no tiene modelo, se muestra un placeholder gris con
// la metadata snapshot del equipo.

import { computed, nextTick, onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'

import { ArrowLeft, Crosshair, History, Lightbulb, Move, Maximize2, Plus, RotateCcw, RotateCw, Trash2, PanelLeftClose, PanelLeftOpen } from '@lucide/vue'

import AppFooter from '@/components/AppFooter.vue'
import AppNavbar from '@/components/AppNavbar.vue'
import BaseButton from '@/components/BaseButton.vue'
import BaseInput from '@/components/BaseInput.vue'
import LabViewer3D, { type LabLighting, type LabModel, type LabTransform, type LabVector, type TransformMode } from '@/components/LabViewer3D.vue'
import { api } from '@/services/api'
import { modelStore } from '@/services/modelStore'
import { useAuthStore } from '@/stores/auth'
import type { EscenaDetail, EscenaInstancia, EscenaLight, Equipo } from '@/services/types'

const route = useRoute()
const auth = useAuthStore()
const { t } = useI18n()

const escenaId = computed(() => String(route.params.id))
const returnRoot = computed(() => typeof route.query.return_root === 'string' ? route.query.return_root : '')
const returnNode = computed(() => typeof route.query.return_node === 'string' ? route.query.return_node : '')
const backToUbicaciones = computed(() => ({
  name: 'ubicaciones',
  query: {
    ...(returnRoot.value ? { root: returnRoot.value } : {}),
    ...(returnNode.value ? { node: returnNode.value } : {}),
  },
}))
const auditRoute = computed(() => ({
  name: 'laboratorio-auditoria',
  params: { id: escenaId.value },
  query: route.query,
}))
const canEdit = computed(() => auth.hasRole('ADMINISTRADOR', 'OPERADOR'))

const escena = ref<EscenaDetail | null>(null)
const equipos = ref<Equipo[]>([])
const loading = ref(false)
const errorMsg = ref<string | null>(null)
const search = ref('')
const sidebarOpen = ref(true)
const activeMode = ref<TransformMode | null>(null)
const labFullscreen = ref(false)
const objectPanelOpen = ref(true)
const rotationAxis = ref<'x' | 'y' | 'z'>('x')
const selectedId = ref<string | null>(null)
const lightSelected = ref(false)
const lightConeVisible = ref(true)
const labSesionId = ref<string | null>(null)
const viewerRef = ref<InstanceType<typeof LabViewer3D> | null>(null)
const objectPanelRef = ref<HTMLElement | null>(null)

// Cache de object URLs resueltos por equipo_origen_id.
// Los liberamos en onBeforeUnmount.
const objectUrls = shallowRef<Map<string, string | null>>(new Map())
const objectUrlRequests = new Map<string, Promise<string | null>>()
const modelUrlErrors = shallowRef<Map<string, string>>(new Map())
const modelLoadErrors = shallowRef<Map<string, string>>(new Map())
const createdUrls: string[] = []
const RAD_TO_DEG = 180 / Math.PI
const DEG_TO_RAD = Math.PI / 180
const rotationAxes = ['x', 'y', 'z'] as const
const MODEL_LOAD_CONCURRENCY = 4
const lightAxes = ['x', 'y', 'z'] as const

function defaultLight(): EscenaLight {
  return {
    activa: false,
    intensidad: 12,
    color: '#fff4d6',
    pos_x: 4,
    pos_y: 6,
    pos_z: 4,
    target_x: 0,
    target_y: 0,
    target_z: 0,
    angulo: 0.55,
    penumbra: 0.35,
    distancia: 30,
    auto_target: false,
  }
}

async function mapLimit<T, R>(
  items: T[],
  limit: number,
  fn: (item: T, index: number) => Promise<R>,
): Promise<R[]> {
  const out = new Array<R>(items.length)
  let next = 0
  const workers = Array.from({ length: Math.min(limit, items.length) }, async () => {
    while (next < items.length) {
      const index = next++
      out[index] = await fn(items[index]!, index)
    }
  })
  await Promise.all(workers)
  return out
}

function setModelUrlError(equipoId: string, message: string | null): void {
  const next = new Map(modelUrlErrors.value)
  if (message) next.set(equipoId, message)
  else next.delete(equipoId)
  modelUrlErrors.value = next
}

function setModelLoadError(instanceId: string, message: string | null): void {
  const next = new Map(modelLoadErrors.value)
  if (message) next.set(instanceId, message)
  else next.delete(instanceId)
  modelLoadErrors.value = next
}

async function resolveModelUrl(equipoId: string | null | undefined): Promise<string | null> {
  if (!equipoId) return null
  if (objectUrls.value.has(equipoId)) return objectUrls.value.get(equipoId) ?? null
  const inFlight = objectUrlRequests.get(equipoId)
  if (inFlight) return inFlight

  const request = (async () => {
    try {
      const url = await modelStore.getObjectUrl(equipoId)
      if (url) {
        if (url.startsWith('blob:')) createdUrls.push(url)
        setModelUrlError(equipoId, null)
      } else {
        setModelUrlError(equipoId, t('equipment.modelRequired'))
      }
      objectUrls.value.set(equipoId, url ?? null)
      return url
    } catch (e) {
      const message = (e as { message?: string }).message ?? t('labs.detail.modelDownloadError')
      objectUrls.value.set(equipoId, null)
      setModelUrlError(equipoId, message)
      return null
    } finally {
      objectUrlRequests.delete(equipoId)
    }
  })()

  objectUrlRequests.set(equipoId, request)
  return request
}

// Modelo de catálogo lateral.
const equiposFiltrados = computed(() => {
  const q = search.value.trim().toLowerCase()
  const base = equipos.value.filter((e) => e.activo)
  if (!q) return base
  return base.filter((e) =>
    e.nombre.toLowerCase().includes(q)
    || (e.fabricante ?? '').toLowerCase().includes(q)
    || (e.modelo ?? '').toLowerCase().includes(q),
  )
})

// `models` reactivo para el LabViewer3D.
const models = ref<LabModel[]>([])

function displayIndexFor(inst: EscenaInstancia): number {
  const index = escena.value?.instancias.findIndex((i) => i.id === inst.id) ?? -1
  return index >= 0 ? index + 1 : inst.orden
}

function labelFor(inst: EscenaInstancia): string {
  return `${inst.nombre_snapshot} - copia ${displayIndexFor(inst)}`
}

function transformFor(inst: EscenaInstancia): LabTransform {
  return {
    x: inst.pos_x,
    y: inst.pos_y,
    z: inst.pos_z,
    scale: inst.escala,
    rotX: inst.rot_x ?? 0,
    rotY: inst.rot_y ?? 0,
    rotZ: inst.rot_z ?? 0,
  }
}

function degrees(v: number | null | undefined): string {
  return (((v ?? 0) * RAD_TO_DEG)).toFixed(0)
}

function fixed(v: number | null | undefined, digits = 2): string {
  return (v ?? 0).toFixed(digits)
}

function modelIssueFor(inst: EscenaInstancia | null): string | null {
  if (!inst) return null
  const runtimeError = modelLoadErrors.value.get(inst.id)
  if (runtimeError) return runtimeError
  if (!inst.equipo_origen_id) return t('labs.detail.instanceMissingOrigin')
  return modelUrlErrors.value.get(inst.equipo_origen_id) ?? null
}

function onModelLoadError(instanceId: string, message: string): void {
  setModelLoadError(instanceId, message)
}

async function rebuildModels(): Promise<void> {
  if (!escena.value) {
    models.value = []
    return
  }
  modelLoadErrors.value = new Map()
  const out = await mapLimit(escena.value.instancias, MODEL_LOAD_CONCURRENCY, async (inst) => {
    const url = await resolveModelUrl(inst.equipo_origen_id)
    return {
      instanceId: inst.id,
      url,
      label: labelFor(inst),
      loadError: modelIssueFor(inst),
      transform: transformFor(inst),
    }
  })
  models.value = out
}

async function load(): Promise<void> {
  loading.value = true
  errorMsg.value = null
  try {
    const det = await api.escenas.get(escenaId.value)
    escena.value = { ...det, iluminacion: det.iluminacion ?? defaultLight() }
    equipos.value = await loadEquiposDisponibles(det)
    await rebuildModels()
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('labs.detail.loadError')
  } finally {
    loading.value = false
  }
}

async function loadEquiposDisponibles(det: EscenaDetail): Promise<Equipo[]> {
  if (!det.nodo_id) return []
  const labNode = await api.nodos.get(det.nodo_id)
  if (!labNode.parent_id) return []
  const [siblings, parentEquipos] = await Promise.all([
    api.nodos.children(labNode.parent_id),
    api.equipos.list({ nodo_id: labNode.parent_id, limit: 200 }),
  ])
  const siblingEquipoNodeIds = new Set(
    (siblings ?? [])
      .filter((n) => n.tipo === 'EQUIPO')
      .map((n) => n.id),
  )
  return (parentEquipos ?? []).filter((e) => !!e.nodo_id && siblingEquipoNodeIds.has(e.nodo_id))
}

function nextPlacement(): Pick<EscenaInstancia, 'pos_x' | 'pos_y' | 'pos_z'> {
  const placed = escena.value?.instancias ?? []
  const occupied = new Set(placed.map((i) => `${i.pos_x.toFixed(2)}:${i.pos_z.toFixed(2)}`))
  const spacing = 2
  const candidates = [
    { pos_x: 0, pos_y: 0, pos_z: 0 },
    { pos_x: spacing, pos_y: 0, pos_z: 0 },
    { pos_x: -spacing, pos_y: 0, pos_z: 0 },
    { pos_x: 0, pos_y: 0, pos_z: spacing },
    { pos_x: 0, pos_y: 0, pos_z: -spacing },
    { pos_x: spacing, pos_y: 0, pos_z: spacing },
    { pos_x: -spacing, pos_y: 0, pos_z: spacing },
    { pos_x: spacing, pos_y: 0, pos_z: -spacing },
    { pos_x: -spacing, pos_y: 0, pos_z: -spacing },
  ]
  const free = candidates.find((p) => !occupied.has(`${p.pos_x.toFixed(2)}:${p.pos_z.toFixed(2)}`))
  if (free) return free

  const ring = Math.floor(placed.length / 8) + 1
  const angle = placed.length * 0.75
  return {
    pos_x: Math.cos(angle) * spacing * ring,
    pos_y: 0,
    pos_z: Math.sin(angle) * spacing * ring,
  }
}

async function agregar(eq: Equipo): Promise<void> {
  if (!escena.value || !canEdit.value) return
  try {
    const inst = await api.escenas.addInstancia(escena.value.id, {
      equipo_id: eq.id,
      lab_sesion_id: labSesionId.value ?? undefined,
      ...nextPlacement(),
    })
    escena.value.instancias = [...escena.value.instancias, inst]
    await rebuildModels()
    selectedId.value = inst.id
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('labs.detail.addError')
  }
}

const selectedInstance = computed<EscenaInstancia | null>(() => {
  if (!escena.value || !selectedId.value) return null
  return escena.value.instancias.find((i) => i.id === selectedId.value) ?? null
})
const objectCount = computed(() => escena.value?.instancias.length ?? 0)

const viewerMode = computed<TransformMode>(() => activeMode.value ?? 'translate')
const viewerEditable = computed(() => canEdit.value && labFullscreen.value && (activeMode.value !== null || lightSelected.value))
const viewerLighting = computed<LabLighting>(() => {
  const l = escena.value?.iluminacion ?? defaultLight()
  return {
    enabled: l.activa,
    intensity: l.intensidad,
    color: l.color,
    position: { x: l.pos_x, y: l.pos_y, z: l.pos_z },
    target: { x: l.target_x, y: l.target_y, z: l.target_z },
    angle: l.angulo,
    penumbra: l.penumbra,
    distance: l.distancia,
    autoTarget: l.auto_target,
  }
})
const selectedRotationPlane = computed(() => {
  if (rotationAxis.value === 'x') return t('labs.transform.plane', { axes: 'YZ' })
  if (rotationAxis.value === 'y') return t('labs.transform.plane', { axes: 'XZ' })
  return t('labs.transform.plane', { axes: 'XY' })
})
const selectedRotationPlaneAxes = computed(() => {
  if (rotationAxis.value === 'x') return ['Y', 'Z']
  if (rotationAxis.value === 'y') return ['X', 'Z']
  return ['X', 'Y']
})

function onSelect(id: string | null): void {
  selectedId.value = id
  if (id) lightSelected.value = false
  if (!id) activeMode.value = null
}

async function selectInstance(id: string): Promise<void> {
  await keepObjectPanelScroll(() => {
    if (selectedId.value === id) {
      selectedId.value = null
      activeMode.value = null
      return
    }
    selectedId.value = id
    lightSelected.value = false
  })
}

async function keepObjectPanelScroll(change: () => void): Promise<void> {
  const panel = objectPanelRef.value
  const scrollTop = panel?.scrollTop ?? 0
  change()
  await nextTick()
  if (objectPanelRef.value) objectPanelRef.value.scrollTop = scrollTop
}

async function toggleMode(next: TransformMode): Promise<void> {
  if (!selectedInstance.value || !canEdit.value) return
  await keepObjectPanelScroll(() => {
    activeMode.value = activeMode.value === next ? null : next
  })
}

async function setRotationAxis(axis: 'x' | 'y' | 'z'): Promise<void> {
  await keepObjectPanelScroll(() => {
    rotationAxis.value = axis
  })
}

function toggleObjectPanel(): void {
  objectPanelOpen.value = !objectPanelOpen.value
}

async function toggleLightSelection(): Promise<void> {
  if (!escena.value?.iluminacion) return
  await keepObjectPanelScroll(() => {
    lightSelected.value = !lightSelected.value
    if (lightSelected.value) activeMode.value = null
  })
}

function onLightSelect(selected: boolean): void {
  lightSelected.value = selected
  if (selected) activeMode.value = null
}

function lightConfig(): EscenaLight | null {
  if (!escena.value) return null
  if (!escena.value.iluminacion) escena.value.iluminacion = defaultLight()
  return escena.value.iluminacion
}

let lightingCommitTimer: number | null = null

function scheduleLightingCommit(): void {
  if (!escena.value || !canEdit.value) return
  if (lightingCommitTimer !== null) window.clearTimeout(lightingCommitTimer)
  lightingCommitTimer = window.setTimeout(async () => {
    if (!escena.value) return
    try {
      const saved = await api.escenas.updateLighting(escena.value.id, escena.value.iluminacion ?? defaultLight())
      escena.value.iluminacion = saved
    } catch (e) {
      errorMsg.value = (e as { message?: string }).message ?? t('labs.lighting.saveError')
    }
  }, 300)
}

function lightOrigin(light: EscenaLight): LabVector {
  return { x: light.pos_x, y: light.pos_y, z: light.pos_z }
}

function lightTarget(light: EscenaLight): LabVector {
  if (light.auto_target && selectedId.value) {
    const center = viewerRef.value?.getObjectCenter(selectedId.value)
    if (center) return center
  }
  return { x: light.target_x, y: light.target_y, z: light.target_z }
}

function vectorDistance(a: LabVector, b: LabVector): number {
  return Math.hypot(a.x - b.x, a.y - b.y, a.z - b.z)
}

function syncLightDistance(light: EscenaLight): void {
  const distance = vectorDistance(lightOrigin(light), lightTarget(light))
  light.distancia = Math.max(0.1, distance)
}

function moveTargetToDistance(light: EscenaLight, distance: number): void {
  if (light.auto_target) {
    light.distancia = distance
    return
  }
  const origin = lightOrigin(light)
  const target = lightTarget(light)
  const dx = target.x - origin.x
  const dy = target.y - origin.y
  const dz = target.z - origin.z
  const length = Math.hypot(dx, dy, dz) || 1
  light.target_x = origin.x + (dx / length) * distance
  light.target_y = origin.y + (dy / length) * distance
  light.target_z = origin.z + (dz / length) * distance
  light.distancia = distance
}

function setLightField<K extends keyof EscenaLight>(field: K, value: EscenaLight[K]): void {
  const light = lightConfig()
  if (!light || !canEdit.value) return
  light[field] = value
  if (field === 'auto_target') syncLightDistance(light)
  scheduleLightingCommit()
}

function setLightNumber(field: keyof EscenaLight, value: number): void {
  if (!Number.isFinite(value)) return
  const light = lightConfig()
  if (!light || !canEdit.value) return
  if (field === 'distancia') {
    setLightDistance(value)
    return
  }
  if (field === 'penumbra' && (value < 0 || value > 1)) return
  ;(light[field] as number) = value
  scheduleLightingCommit()
}

function setLightDistance(value: number): void {
  if (!Number.isFinite(value) || value <= 0) return
  const light = lightConfig()
  if (!light || !canEdit.value) return
  moveTargetToDistance(light, value)
  scheduleLightingCommit()
}

function setLightTarget(axis: 'x' | 'y' | 'z', value: number): void {
  const field = `target_${axis}` as keyof EscenaLight
  const light = lightConfig()
  if (light) light.auto_target = false
  setLightNumber(field, value)
  if (light) syncLightDistance(light)
}

function setLightAngle(deg: number): void {
  if (!Number.isFinite(deg)) return
  const clamped = Math.min(90, Math.max(1, deg))
  setLightNumber('angulo', clamped * DEG_TO_RAD)
}

function pointLightAtSelected(): void {
  const light = lightConfig()
  if (!light || !selectedId.value || !canEdit.value) return
  const center = viewerRef.value?.getObjectCenter(selectedId.value) ?? (
    selectedInstance.value
      ? { x: selectedInstance.value.pos_x, y: selectedInstance.value.pos_y, z: selectedInstance.value.pos_z }
      : null
  )
  if (!center) return
  light.target_x = center.x
  light.target_y = center.y
  light.target_z = center.z
  light.auto_target = false
  syncLightDistance(light)
  scheduleLightingCommit()
}

function onLightOriginChange(position: LabVector): void {
  const light = lightConfig()
  if (!light || !canEdit.value) return
  light.pos_x = position.x
  light.pos_y = position.y
  light.pos_z = position.z
  syncLightDistance(light)
}

function onLightOriginCommit(position: LabVector): void {
  onLightOriginChange(position)
  scheduleLightingCommit()
}

function restoreLight(): void {
  const light = lightConfig()
  if (!light || !canEdit.value) return
  Object.assign(light, {
    ...defaultLight(),
    activa: true,
  })
  lightSelected.value = true
  activeMode.value = null
  scheduleLightingCommit()
}

function lightAngleDegrees(light: EscenaLight): string {
  return degrees(light.angulo)
}

function rotationValue(inst: EscenaInstancia, axis: 'x' | 'y' | 'z'): string {
  if (axis === 'x') return degrees(inst.rot_x)
  if (axis === 'y') return degrees(inst.rot_y)
  return degrees(inst.rot_z)
}

async function startLabSession(): Promise<boolean> {
  if (!escena.value) return false
  if (labSesionId.value) return true
  try {
    const sesion = await api.escenas.startSesion(escena.value.id)
    labSesionId.value = sesion.id
    return true
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('labs.detail.startSessionError')
    return false
  }
}

async function closeLabSession(motivo = 'manual'): Promise<void> {
  const id = labSesionId.value
  const labId = escena.value?.id
  if (!id || !labId) return
  try {
    await flushTransformCommit()
    await api.escenas.closeSesion(labId, id, motivo)
    labSesionId.value = null
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('labs.detail.closeSessionError')
    labSesionId.value = null
  }
}

async function enterLaboratorio(): Promise<void> {
  if (!(await startLabSession())) return
  labFullscreen.value = true
  objectPanelOpen.value = true
  activeMode.value = null
  await nextTick()
  centrar()
}

async function exitLaboratorio(): Promise<void> {
  await closeLabSession('salida')
  activeMode.value = null
  lightSelected.value = false
  labFullscreen.value = false
  await nextTick()
  centrar()
}

function onKeydown(ev: KeyboardEvent): void {
  if (ev.key === 'Escape' && labFullscreen.value) {
    void exitLaboratorio()
  }
}

function onPageHide(): void {
  if (labSesionId.value) void closeLabSession('navegador')
}

// Commit del transform al backend.
let commitTimer: number | null = null
const pendingCommits = new Map<string, LabTransform>()

async function persistTransform(instId: string, t: LabTransform): Promise<void> {
  if (!escena.value || !canEdit.value) return
  await api.escenas.updateInstancia(escena.value.id, instId, {
    lab_sesion_id: labSesionId.value ?? undefined,
    pos_x: t.x, pos_y: t.y, pos_z: t.z, escala: t.scale,
    rot_x: t.rotX, rot_y: t.rotY, rot_z: t.rotZ,
  })
}

async function flushTransformCommit(): Promise<void> {
  if (commitTimer !== null) {
    window.clearTimeout(commitTimer)
    commitTimer = null
  }
  const pending = Array.from(pendingCommits.entries())
  pendingCommits.clear()
  if (pending.length === 0) return
  try {
    for (const [instId, t] of pending) {
      await persistTransform(instId, t)
    }
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('labs.transform.saveError')
  }
}

async function onTransformCommit(
  instId: string,
  transform: LabTransform,
): Promise<void> {
  if (!escena.value || !canEdit.value) return
  // Optimistic update del estado local para que la UI no parpadee.
  const inst = escena.value.instancias.find((i) => i.id === instId)
  if (inst) {
    inst.pos_x = transform.x; inst.pos_y = transform.y; inst.pos_z = transform.z; inst.escala = transform.scale
    inst.rot_x = transform.rotX; inst.rot_y = transform.rotY; inst.rot_z = transform.rotZ
  }
  if (commitTimer !== null) window.clearTimeout(commitTimer)
  pendingCommits.set(instId, transform)
  commitTimer = window.setTimeout(async () => {
    try {
      const pending = Array.from(pendingCommits.entries())
      pendingCommits.clear()
      commitTimer = null
      for (const [pendingInstId, pendingTransform] of pending) {
        await persistTransform(pendingInstId, pendingTransform)
      }
    } catch (e) {
      errorMsg.value = (e as { message?: string }).message ?? t('labs.transform.saveError')
    }
  }, 300)
}

// Edicion directa desde el panel de laboratorio.
async function setPos(axis: 'x' | 'y' | 'z', v: number): Promise<void> {
  const inst = selectedInstance.value
  if (!inst || !canEdit.value) return
  if (!Number.isFinite(v)) return
  if (axis === 'x') inst.pos_x = v
  else if (axis === 'y') inst.pos_y = v
  else inst.pos_z = v
  await rebuildModels()
  await onTransformCommit(inst.id, transformFor(inst))
}

async function setEscala(v: number): Promise<void> {
  const inst = selectedInstance.value
  if (!inst || !canEdit.value) return
  if (!Number.isFinite(v) || v <= 0) return
  inst.escala = v
  await rebuildModels()
  await onTransformCommit(inst.id, transformFor(inst))
}

async function setRot(axis: 'x' | 'y' | 'z', deg: number): Promise<void> {
  const inst = selectedInstance.value
  if (!inst || !canEdit.value) return
  if (!Number.isFinite(deg)) return
  const rad = deg * DEG_TO_RAD
  if (axis === 'x') inst.rot_x = rad
  else if (axis === 'y') inst.rot_y = rad
  else inst.rot_z = rad
  await rebuildModels()
  await onTransformCommit(inst.id, transformFor(inst))
}

async function restoreInitial(): Promise<void> {
  const inst = selectedInstance.value
  if (!inst || !escena.value || !canEdit.value) return
  if (!confirm(t('labs.transform.restoreInitialConfirm', { name: labelFor(inst) }))) return
  try {
    await flushTransformCommit()
    const upd = await api.escenas.restoreInstancia(escena.value.id, inst.id, labSesionId.value)
    Object.assign(inst, upd)
    await rebuildModels()
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('labs.transform.restoreError')
  }
}

async function restoreFromLastSession(): Promise<void> {
  const inst = selectedInstance.value
  if (!inst || !escena.value || !canEdit.value) return
  try {
    await flushTransformCommit()
    const upd = await api.escenas.restoreInstanciaSesion(escena.value.id, inst.id, labSesionId.value)
    Object.assign(inst, upd)
    await rebuildModels()
  } catch (e) {
    const msg = (e as { message?: string }).message
    errorMsg.value = msg === 'recurso no encontrado'
      ? t('labs.transform.noPreviousSession')
      : msg ?? t('labs.transform.restorePreviousError')
  }
}

async function eliminar(): Promise<void> {
  const inst = selectedInstance.value
  if (!inst || !escena.value || !canEdit.value) return
  if (!confirm(t('labs.detail.removeConfirm', { name: labelFor(inst) }))) return
  try {
    await api.escenas.removeInstancia(escena.value.id, inst.id)
    escena.value.instancias = escena.value.instancias.filter((i) => i.id !== inst.id)
    selectedId.value = null
    activeMode.value = null
    await rebuildModels()
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('labs.detail.removeError')
  }
}

function centrar(): void { viewerRef.value?.frameAll() }

onMounted(() => {
  void load()
  window.addEventListener('keydown', onKeydown)
  window.addEventListener('pagehide', onPageHide)
})

watch(escenaId, async () => { await load() })

watch(labFullscreen, async (value) => {
  document.body.style.overflow = value ? 'hidden' : ''
  await nextTick()
  centrar()
})

onBeforeUnmount(() => {
  void closeLabSession('unmount')
  if (commitTimer !== null) window.clearTimeout(commitTimer)
  if (lightingCommitTimer !== null) window.clearTimeout(lightingCommitTimer)
  window.removeEventListener('keydown', onKeydown)
  window.removeEventListener('pagehide', onPageHide)
  document.body.style.overflow = ''
  // Revocar object URLs locales para no fugar memoria.
  for (const u of createdUrls) {
    try { URL.revokeObjectURL(u) } catch { /* noop */ }
  }
})
</script>

<template>
  <div class="app-shell">
    <AppNavbar />
    <main class="lab-page">
      <header class="lab-head">
        <RouterLink class="back" :to="backToUbicaciones">
          <ArrowLeft :size="16" aria-hidden="true" /> {{ $t('common.back') }}
        </RouterLink>
        <h1 v-if="escena">{{ escena.nombre }}</h1>
        <h1 v-else>{{ $t('common.loading') }}</h1>
        <button type="button" class="ico-btn" @click="sidebarOpen = !sidebarOpen" :title="sidebarOpen ? $t('labs.detail.hideCatalog') : $t('labs.detail.showCatalog')">
          <PanelLeftClose v-if="sidebarOpen" :size="18" aria-hidden="true" />
          <PanelLeftOpen v-else :size="18" aria-hidden="true" />
        </button>
      </header>

      <p v-if="errorMsg" class="err">{{ errorMsg }}</p>

      <div class="lab-layout" :class="{ 'lab-layout--collapsed': !sidebarOpen }">
        <aside v-if="sidebarOpen" class="lab-side">
          <h2>{{ $t('labs.detail.locationEquipment') }}</h2>
          <BaseInput v-model="search" :placeholder="$t('labs.detail.searchEquipment')" />
          <ul class="cat-list">
            <li v-for="eq in equiposFiltrados" :key="eq.id" class="cat-item">
              <div class="cat-meta">
                <strong>{{ eq.nombre }}</strong>
                <small v-if="eq.fabricante || eq.modelo">
                  {{ [eq.fabricante, eq.modelo].filter(Boolean).join(' · ') }}
                </small>
              </div>
              <button
                v-if="canEdit"
                class="btn-action btn-add"
                type="button"
                @click="agregar(eq)"
                :title="$t('labs.detail.addEquipment')"
              >
                <Plus :size="14" aria-hidden="true" /> {{ $t('common.add') }}
              </button>
            </li>
            <li v-if="equiposFiltrados.length === 0" class="muted small">{{ $t('equipment.empty') }}</li>
          </ul>
        </aside>

        <div
          class="lab-session"
          :class="{
            'lab-session--immersive': labFullscreen,
            'lab-session--preview': !labFullscreen,
            'lab-session--panel-collapsed': labFullscreen && !objectPanelOpen,
          }"
        >
          <aside v-if="labFullscreen && escena" v-show="objectPanelOpen" ref="objectPanelRef" class="lab-object-panel">
            <header class="object-panel-head">
              <small>{{ $t('labs.detail.activeLab') }}</small>
              <strong>{{ escena.nombre }}</strong>
              <span>{{ $t('labs.detail.objectCount', { count: objectCount }) }}</span>
            </header>

            <section class="light-section" :class="{ 'light-section--active': lightSelected }" v-if="escena.iluminacion">
              <button
                type="button"
                class="section-title section-title--button"
                :class="{ active: lightSelected }"
                @click="toggleLightSelection"
              >
                <h2>{{ $t('labs.lighting.title') }}</h2>
                <Lightbulb :size="15" aria-hidden="true" />
              </button>

              <div v-if="lightSelected" class="light-panel-body">
                <label class="switch-row">
                  <span>{{ $t('labs.lighting.showCone') }}</span>
                  <input
                    type="checkbox"
                    v-model="lightConeVisible"
                  />
                </label>

                <label class="switch-row">
                  <span>{{ $t('labs.lighting.focus') }}</span>
                  <input
                    type="checkbox"
                    :checked="escena.iluminacion.activa"
                    :disabled="!canEdit"
                    @change="setLightField('activa', ($event.target as HTMLInputElement).checked)"
                  />
                </label>

                <div class="light-grid">
                  <label class="light-control light-control--wide">
                    {{ $t('labs.lighting.intensity') }}
                    <input
                      type="range"
                      min="0"
                      max="60"
                      step="0.1"
                      :value="escena.iluminacion.intensidad"
                      :disabled="!canEdit"
                      @input="setLightNumber('intensidad', Number(($event.target as HTMLInputElement).value))"
                    />
                    <span>{{ fixed(escena.iluminacion.intensidad, 1) }}</span>
                  </label>

                  <label class="light-control">
                    {{ $t('labs.lighting.color') }}
                    <input
                      type="color"
                      :value="escena.iluminacion.color"
                      :disabled="!canEdit"
                      @input="setLightField('color', ($event.target as HTMLInputElement).value)"
                    />
                  </label>

                  <label class="light-control">
                    {{ $t('labs.lighting.angle') }}
                    <input
                      type="number"
                      min="1"
                      max="90"
                      step="1"
                      :value="lightAngleDegrees(escena.iluminacion)"
                      :disabled="!canEdit"
                      @change="setLightAngle(Number(($event.target as HTMLInputElement).value))"
                    />
                  </label>

                  <label class="light-control">
                    {{ $t('labs.lighting.distance') }}
                    <input
                      type="number"
                      min="1"
                      step="1"
                      :value="fixed(escena.iluminacion.distancia, 0)"
                      :disabled="!canEdit"
                      @change="setLightDistance(Number(($event.target as HTMLInputElement).value))"
                    />
                  </label>

                  <label class="light-control">
                    {{ $t('labs.lighting.penumbra') }}
                    <input
                      type="number"
                      min="0"
                      max="1"
                      step="0.05"
                      :value="fixed(escena.iluminacion.penumbra, 2)"
                      :disabled="!canEdit"
                      @change="setLightNumber('penumbra', Number(($event.target as HTMLInputElement).value))"
                    />
                  </label>
                </div>

                <div class="light-origin-readout">
                  <span>{{ $t('labs.lighting.origin') }}</span>
                  <small>
                    X {{ fixed(escena.iluminacion.pos_x) }} ·
                    Y {{ fixed(escena.iluminacion.pos_y) }} ·
                    Z {{ fixed(escena.iluminacion.pos_z) }}
                  </small>
                </div>

                <label class="switch-row">
                  <span>{{ $t('labs.lighting.followSelected') }}</span>
                  <input
                    type="checkbox"
                    :checked="escena.iluminacion.auto_target"
                    :disabled="!canEdit"
                    @change="setLightField('auto_target', ($event.target as HTMLInputElement).checked)"
                  />
                </label>

                <div class="light-fields">
                  <span>{{ $t('labs.lighting.target') }}</span>
                  <label v-for="axis in lightAxes" :key="`target-${axis}`" class="mini-input">
                    {{ axis.toUpperCase() }}
                    <input
                      type="number"
                      step="0.25"
                      :value="fixed(escena.iluminacion[`target_${axis}`])"
                      :disabled="!canEdit || escena.iluminacion.auto_target"
                      @change="setLightTarget(axis, Number(($event.target as HTMLInputElement).value))"
                    />
                  </label>
                </div>

                <div class="tool-actions tool-actions--light">
                  <button
                    class="btn-action"
                    type="button"
                    :disabled="!canEdit || !selectedInstance"
                    @click="pointLightAtSelected"
                  >
                    <Crosshair :size="14" aria-hidden="true" /> {{ $t('labs.lighting.setTarget') }}
                  </button>
                  <button class="btn-action" type="button" :disabled="!canEdit" @click="restoreLight">
                    <RotateCcw :size="14" aria-hidden="true" /> {{ $t('labs.lighting.restoreLight') }}
                  </button>
                </div>
              </div>
            </section>

            <section class="object-section">
              <h2>{{ $t('labs.detail.presentObjects', { count: objectCount }) }}</h2>
              <div class="object-list" v-if="objectCount > 0">
                <button
                  v-for="(inst, index) in escena.instancias"
                  :key="inst.id"
                  type="button"
                  class="object-item"
                  :class="{ active: selectedId === inst.id }"
                  @click="selectInstance(inst.id)"
                >
                  <span class="object-index">{{ index + 1 }}</span>
                  <span class="object-text">
                    <strong>{{ inst.nombre_snapshot }}</strong>
                    <small v-if="modelIssueFor(inst)" class="model-warning">{{ modelIssueFor(inst) }}</small>
                    <small>{{ [inst.fabricante_snapshot, inst.modelo_snapshot].filter(Boolean).join(' · ') || $t('labs.detail.noMetadata') }}</small>
                  </span>
                </button>
              </div>
              <p v-else class="muted small">{{ $t('labs.detail.noObjects') }}</p>
            </section>

            <section class="object-tools" v-if="selectedInstance && !lightSelected">
              <div class="selected-card">
                <small>{{ $t('common.selected') }}</small>
                <strong>{{ labelFor(selectedInstance) }}</strong>
                <span v-if="modelIssueFor(selectedInstance)" class="model-warning">{{ modelIssueFor(selectedInstance) }}</span>
                <span v-if="selectedInstance.categoria_snapshot">{{ selectedInstance.categoria_snapshot }}</span>
              </div>

              <div v-if="canEdit" class="tool-stack">
                <button
                  type="button"
                  class="tool-toggle"
                  :class="{ active: activeMode === 'translate' }"
                  @click="toggleMode('translate')"
                >
                  <Move :size="15" aria-hidden="true" /> {{ $t('labs.transform.translate') }}
                </button>
                <div v-if="activeMode === 'translate'" class="tool-fields">
                  <label class="transform-input">
                    X
                    <input type="number" step="0.1" :value="fixed(selectedInstance.pos_x)" @change="setPos('x', Number(($event.target as HTMLInputElement).value))" />
                  </label>
                  <label class="transform-input">
                    Y
                    <input type="number" step="0.1" :value="fixed(selectedInstance.pos_y)" @change="setPos('y', Number(($event.target as HTMLInputElement).value))" />
                  </label>
                  <label class="transform-input">
                    Z
                    <input type="number" step="0.1" :value="fixed(selectedInstance.pos_z)" @change="setPos('z', Number(($event.target as HTMLInputElement).value))" />
                  </label>
                </div>

                <button
                  type="button"
                  class="tool-toggle"
                  :class="{ active: activeMode === 'rotate' }"
                  @click="toggleMode('rotate')"
                >
                  <RotateCw :size="15" aria-hidden="true" /> {{ $t('labs.transform.rotate') }}
                </button>
                <div v-if="activeMode === 'rotate'" class="tool-fields tool-fields--rotation">
                  <div class="axis-tabs" role="radiogroup" :aria-label="$t('labs.transform.rotationAxis')">
                    <button
                      v-for="axis in rotationAxes"
                      :key="axis"
                      type="button"
                      class="axis-tab"
                      :class="{ active: rotationAxis === axis }"
                      @click="setRotationAxis(axis)"
                    >
                      {{ $t('labs.transform.axis', { axis: axis.toUpperCase() }) }}
                      <strong>{{ rotationValue(selectedInstance, axis) }}°</strong>
                    </button>
                  </div>
                  <div class="rotation-plane" :class="`rotation-plane--${rotationAxis}`">
                    <span class="plane-label">{{ selectedRotationPlane }}</span>
                    <span class="plane-axis plane-axis--a">{{ selectedRotationPlaneAxes[0] }}</span>
                    <span class="plane-axis plane-axis--b">{{ selectedRotationPlaneAxes[1] }}</span>
                    <span class="plane-silhouette" />
                  </div>
                  <label class="transform-input transform-input--wide">
                    {{ $t('labs.transform.rotationLabel', { axis: rotationAxis.toUpperCase() }) }}
                    <input
                      type="number"
                      step="5"
                      :value="rotationValue(selectedInstance, rotationAxis)"
                      @change="setRot(rotationAxis, Number(($event.target as HTMLInputElement).value))"
                    />
                  </label>
                </div>

                <button
                  type="button"
                  class="tool-toggle"
                  :class="{ active: activeMode === 'scale' }"
                  @click="toggleMode('scale')"
                >
                  <Maximize2 :size="15" aria-hidden="true" /> {{ $t('labs.transform.scale') }}
                </button>
                <div v-if="activeMode === 'scale'" class="tool-fields">
                  <label class="transform-input transform-input--wide">
                    {{ $t('labs.transform.factor') }}
                    <input type="number" min="0.05" step="0.05" :value="fixed(selectedInstance.escala)" @change="setEscala(Number(($event.target as HTMLInputElement).value))" />
                  </label>
                </div>

                <div class="tool-actions">
                  <button class="btn-action" type="button" @click="restoreInitial" :title="$t('labs.transform.restoreInitialTitle')">
                    <RotateCcw :size="14" aria-hidden="true" /> {{ $t('labs.transform.initial') }}
                  </button>
                  <button class="btn-action" type="button" @click="restoreFromLastSession" :title="$t('labs.transform.restorePreviousTitle')">
                    <RotateCcw :size="14" aria-hidden="true" /> {{ $t('labs.transform.previousSession') }}
                  </button>
                  <button class="btn-action btn-delete" type="button" @click="eliminar">
                    <Trash2 :size="14" aria-hidden="true" /> {{ $t('common.remove') }}
                  </button>
                </div>
              </div>

              <p v-else class="muted small">{{ $t('common.readOnly') }}</p>
            </section>
            <section v-else-if="!lightSelected" class="object-tools object-tools--empty">
              <strong>{{ $t('common.noSelection') }}</strong>
              <small>{{ $t('labs.detail.noSelectionHint') }}</small>
            </section>
          </aside>

          <section class="lab-stage">
            <div class="stage-wrap">
              <LabViewer3D
                ref="viewerRef"
                :models="models"
                :mode="viewerMode"
                :editable="viewerEditable"
                :lighting="viewerLighting"
                :active-axis="activeMode === 'rotate' ? rotationAxis : null"
                :selected-id="selectedId"
                :light-selected="lightSelected"
                :show-light-cone="lightConeVisible"
                @select="onSelect"
                @light-select="onLightSelect"
                @light-origin-change="onLightOriginChange"
                @light-origin-commit="onLightOriginCommit"
                @model-load-error="onModelLoadError"
                @transform-commit="onTransformCommit"
              />

              <button v-if="!labFullscreen" class="enter-lab-btn" type="button" @click="enterLaboratorio">
                {{ $t('labs.detail.enterLab') }}
              </button>

              <div v-if="labFullscreen" class="lab-topbar">
                <button class="top-chip" type="button" @click="exitLaboratorio">
                  <ArrowLeft :size="15" aria-hidden="true" /> {{ $t('nodeTypes.LABORATORIO') }}
                </button>
                <button class="top-chip" type="button" @click="toggleObjectPanel">
                  <PanelLeftClose v-if="objectPanelOpen" :size="15" aria-hidden="true" />
                  <PanelLeftOpen v-else :size="15" aria-hidden="true" />
                  {{ $t('labs.detail.objects') }}
                </button>
                <button class="top-chip" type="button" @click="centrar">{{ $t('common.center') }}</button>
              </div>
            </div>

            <div class="preview-strip" v-if="!labFullscreen && escena">
              <div>
                <strong>{{ $t('labs.detail.preview') }}</strong>
                <small>{{ $t('labs.detail.placedObjects', { count: objectCount }) }}</small>
              </div>
              <div class="preview-actions">
                <RouterLink class="preview-action" :to="auditRoute">
                  <History :size="16" aria-hidden="true" /> {{ $t('common.audit') }}
                </RouterLink>
                <BaseButton variant="ghost" @click="centrar">{{ $t('common.centerCamera') }}</BaseButton>
              </div>
            </div>
          </section>
        </div>
      </div>
    </main>
    <AppFooter />
  </div>
</template>

<style scoped>
.lab-page {
  width: min(1800px, calc(100vw - 1.5rem));
  max-width: none;
  margin: 0 auto;
  padding: 0.85rem 0;
}
.lab-head {
  position: relative;
  z-index: 3;
  display: flex; align-items: center; gap: 1rem; margin-bottom: 0.75rem;
}
.lab-head h1 { margin: 0; flex: 1; font-size: 1.3rem; }
.back {
  display: inline-flex; align-items: center; gap: 0.35rem;
  background: none; border: 0; cursor: pointer; color: var(--c-text-muted);
  font: inherit;
  text-decoration: none;
}
.back:hover { color: var(--c-text); }
.ico-btn {
  background: var(--c-surface-2); color: var(--c-text);
  border: 1px solid var(--c-border); border-radius: var(--radius-sm);
  padding: 0.35rem 0.55rem; cursor: pointer;
}
.ico-btn:hover { background: var(--c-surface-3); }

.lab-layout {
  display: grid;
  grid-template-columns: clamp(300px, 24vw, 380px) minmax(0, 1fr);
  gap: 0.75rem;
  min-height: 82vh;
}
.lab-layout--collapsed { grid-template-columns: 1fr; }

.lab-session {
  min-width: 0;
}
.lab-session--preview {
  display: block;
}
.lab-session--immersive {
  position: fixed;
  inset: 0;
  z-index: 1000;
  display: grid;
  grid-template-columns: minmax(290px, 340px) minmax(0, 1fr);
  min-height: 100dvh;
  background: #070a0f;
  color: #eef3f8;
}
.lab-session--panel-collapsed {
  grid-template-columns: minmax(0, 1fr);
}

.lab-side {
  position: relative;
  z-index: 2;
  min-width: 0;
  background: var(--c-surface);
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  padding: 0.7rem;
  display: flex; flex-direction: column; gap: 0.5rem;
  max-height: 82vh; overflow: auto;
}
.lab-side h2 { margin: 0; font-size: 0.95rem; color: var(--c-text-muted); text-transform: uppercase; letter-spacing: 0.05em; }
.cat-list { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.35rem; }
.cat-item {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 0.65rem;
  padding: 0.5rem 0.6rem;
  background: var(--c-surface-2);
  border: 1px solid var(--c-border);
  border-radius: var(--radius-sm);
}
.cat-meta {
  min-width: 0;
  display: flex;
  flex-direction: column;
  line-height: 1.15;
}
.cat-meta strong,
.cat-meta small {
  overflow-wrap: anywhere;
}
.cat-meta strong { font-size: 0.92rem; }
.cat-meta small { color: var(--c-text-muted); font-size: 0.78rem; }

.lab-stage {
  display: grid;
  grid-template-rows: 1fr auto;
  gap: 0.75rem;
  min-height: 82vh;
}
.lab-session--immersive .lab-stage {
  min-height: 100dvh;
  height: 100dvh;
  gap: 0;
}
.stage-wrap {
  position: relative;
  z-index: 1;
  min-height: 74vh;
}
.lab-session--immersive .stage-wrap {
  min-height: 0;
  height: 100dvh;
}
.lab-session--immersive :deep(.lab-viewer) {
  border: 0;
  border-radius: 0;
}

.enter-lab-btn {
  position: absolute;
  left: 50%;
  bottom: 1rem;
  z-index: 5;
  transform: translateX(-50%);
  border: 1px solid rgba(255,255,255,0.22);
  border-radius: 999px;
  background: rgba(12, 17, 24, 0.82);
  color: #fff;
  padding: 0.55rem 0.9rem;
  font: inherit;
  font-size: 0.9rem;
  cursor: pointer;
  box-shadow: 0 12px 30px rgba(0,0,0,0.25);
  backdrop-filter: blur(8px);
}
.enter-lab-btn:hover { background: rgba(23, 31, 43, 0.92); }

.preview-strip {
  position: relative;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  background: var(--c-surface);
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  padding: 0.75rem 1rem;
}
.preview-strip div { display: flex; flex-direction: column; gap: 0.15rem; }
.preview-strip small { color: var(--c-text-muted); }
.preview-actions {
  display: flex;
  flex-direction: row !important;
  align-items: center;
  justify-content: flex-end;
  gap: 0.45rem !important;
  min-width: 0;
}
.preview-action {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  min-height: 40px;
  padding: 0.55rem 0.9rem;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  background: var(--c-surface-2);
  color: var(--c-text);
  font: inherit;
  font-weight: 600;
  text-decoration: none;
}
.preview-action:hover { background: var(--c-surface-3); }

.lab-topbar {
  position: absolute;
  top: 0.85rem;
  left: 0.85rem;
  right: 0.85rem;
  z-index: 6;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  pointer-events: none;
}
.top-chip {
  pointer-events: auto;
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  min-height: 34px;
  padding: 0.4rem 0.65rem;
  border: 1px solid rgba(255,255,255,0.16);
  border-radius: var(--radius-sm);
  background: rgba(10, 14, 20, 0.74);
  color: #f8fafc;
  text-decoration: none;
  font: inherit;
  font-size: 0.84rem;
  cursor: pointer;
  backdrop-filter: blur(8px);
}
.top-chip:hover { background: rgba(30, 41, 59, 0.88); }
.top-chip:last-child { margin-left: auto; }

.lab-object-panel {
  position: relative;
  z-index: 7;
  height: 100dvh;
  overflow: auto;
  display: flex;
  flex-direction: column;
  gap: 0.9rem;
  padding: 1rem;
  background: #101720;
  border-right: 1px solid rgba(255,255,255,0.1);
  box-shadow: 16px 0 40px rgba(0,0,0,0.28);
}
.object-panel-head {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  padding-bottom: 0.8rem;
  border-bottom: 1px solid rgba(255,255,255,0.1);
}
.object-panel-head small,
.object-panel-head span {
  color: #9ca9b8;
  font-size: 0.78rem;
}
.object-panel-head strong { font-size: 1.02rem; }

.section-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.55rem;
  color: #9ca9b8;
}
.section-title--button {
  width: 100%;
  min-height: 38px;
  padding: 0.55rem 0.65rem;
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: var(--radius-sm);
  background: #151f2a;
  font: inherit;
  cursor: pointer;
}
.section-title--button:hover { background: #1b2836; }
.section-title--button.active {
  color: #fff7d6;
  border-color: #fbbf24;
  background: #3c2b12;
}
.section-title h2 {
  margin: 0;
  font-size: 0.78rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
.light-section {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
  padding-bottom: 0.9rem;
  border-bottom: 1px solid rgba(255,255,255,0.1);
}
.light-panel-body {
  display: flex;
  flex-direction: column;
  gap: 0.55rem;
}
.switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  min-height: 34px;
  color: #dbe7f4;
  font-size: 0.84rem;
}
.switch-row input {
  width: 18px;
  height: 18px;
  accent-color: #f59e0b;
}
.light-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.45rem;
}
.light-control {
  display: flex;
  flex-direction: column;
  gap: 0.28rem;
  min-width: 0;
  color: #9ca9b8;
  font-size: 0.76rem;
}
.light-control--wide {
  grid-column: 1 / -1;
}
.light-control input {
  width: 100%;
  min-height: 32px;
  padding: 0.25rem 0.4rem;
  background: #0c1219;
  color: #f8fafc;
  border: 1px solid rgba(255,255,255,0.12);
  border-radius: var(--radius-sm);
  font: inherit;
}
.light-control input[type="range"] {
  padding: 0;
  accent-color: #f59e0b;
}
.light-control input[type="color"] {
  padding: 0.16rem;
}
.light-control span {
  align-self: flex-end;
  color: #e7edf5;
  font-size: 0.76rem;
}
.light-fields {
  display: grid;
  grid-template-columns: 64px repeat(3, 1fr);
  align-items: center;
  gap: 0.35rem;
}
.light-fields > span {
  color: #9ca9b8;
  font-size: 0.76rem;
}
.light-origin-readout {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  min-height: 34px;
  padding: 0.45rem 0.55rem;
  background: #0c1219;
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: var(--radius-sm);
}
.light-origin-readout span {
  color: #9ca9b8;
  font-size: 0.76rem;
}
.light-origin-readout small {
  color: #e7edf5;
  font-size: 0.76rem;
  text-align: right;
}
.mini-input {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  color: #aeb9c7;
  font-size: 0.74rem;
}
.mini-input input {
  min-width: 0;
  width: 100%;
  padding: 0.28rem 0.32rem;
  background: #0c1219;
  color: #f8fafc;
  border: 1px solid rgba(255,255,255,0.12);
  border-radius: var(--radius-sm);
  font: inherit;
}

.object-section h2 {
  margin: 0 0 0.55rem;
  color: #9ca9b8;
  font-size: 0.78rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
}
.object-list {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}
.object-item {
  display: grid;
  grid-template-columns: 30px 1fr;
  align-items: center;
  gap: 0.55rem;
  width: 100%;
  padding: 0.55rem;
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: var(--radius-sm);
  background: #151f2a;
  color: #e7edf5;
  text-align: left;
  font: inherit;
  cursor: pointer;
}
.object-item:hover { background: #1b2836; }
.object-item.active {
  border-color: #60a5fa;
  background: #172b42;
}
.object-index {
  display: grid;
  place-items: center;
  width: 30px;
  height: 30px;
  border-radius: 50%;
  background: #263445;
  color: #cfe0f4;
  font-size: 0.78rem;
  font-weight: 800;
}
.object-text {
  display: flex;
  flex-direction: column;
  gap: 0.12rem;
  min-width: 0;
}
.object-text strong,
.object-text small {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.object-text strong { font-size: 0.88rem; }
.object-text small { color: #9ca9b8; font-size: 0.75rem; }
.model-warning {
  color: #fbbf24 !important;
  white-space: normal !important;
  overflow: visible !important;
  text-overflow: initial !important;
}

.object-tools {
  display: flex;
  flex-direction: column;
  gap: 0.7rem;
  margin-top: auto;
  padding-top: 0.9rem;
  border-top: 1px solid rgba(255,255,255,0.1);
}
.object-tools--empty {
  margin-top: 0;
  color: #c7d2df;
}
.object-tools--empty small { color: #9ca9b8; }
.selected-card {
  display: flex;
  flex-direction: column;
  gap: 0.16rem;
  padding: 0.65rem;
  background: #0c1219;
  border: 1px solid rgba(255,255,255,0.09);
  border-radius: var(--radius-sm);
}
.selected-card small,
.selected-card span {
  color: #9ca9b8;
  font-size: 0.76rem;
}
.selected-card strong { font-size: 0.9rem; }

.tool-stack {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
}
.tool-toggle {
  display: flex;
  align-items: center;
  gap: 0.45rem;
  width: 100%;
  min-height: 38px;
  padding: 0.55rem 0.65rem;
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: var(--radius-sm);
  background: #151f2a;
  color: #e7edf5;
  font: inherit;
  font-size: 0.88rem;
  cursor: pointer;
}
.tool-toggle:hover { background: #1b2836; }
.tool-toggle.active {
  color: #fff;
  border-color: #60a5fa;
  background: #1d4f7a;
}
.tool-fields {
  display: grid;
  gap: 0.5rem;
  padding: 0.65rem;
  background: #0c1219;
  border: 1px solid rgba(255,255,255,0.08);
  border-radius: var(--radius-sm);
}

.transform-fields {
  display: inline-flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.45rem;
  padding: 0.35rem 0.45rem;
  background: var(--c-surface-2);
  border: 1px solid var(--c-border);
  border-radius: var(--radius-sm);
}
.transform-fields__title {
  margin-right: 0.2rem;
  font-size: 0.8rem;
  font-weight: 700;
  color: var(--c-text);
}
.transform-input {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.45rem;
  font-size: 0.85rem;
  color: #aeb9c7;
}
.transform-input input {
  width: 80px; padding: 0.3rem 0.45rem;
  background: #101720; color: #f8fafc;
  border: 1px solid rgba(255,255,255,0.14); border-radius: var(--radius-sm);
  font: inherit;
}
.transform-input--wide input { width: 96px; }

.axis-tabs {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 0.35rem;
}
.axis-tab {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.12rem;
  min-height: 48px;
  padding: 0.4rem 0.2rem;
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: var(--radius-sm);
  background: #151f2a;
  color: #dbe7f4;
  font: inherit;
  font-size: 0.75rem;
  cursor: pointer;
}
.axis-tab strong { font-size: 0.82rem; }
.axis-tab:hover { background: #1b2836; }
.axis-tab.active {
  border-color: #fbbf24;
  background: #3c2b12;
  color: #fff7d6;
}
.rotation-plane {
  position: relative;
  height: 138px;
  overflow: hidden;
  border: 1px solid rgba(255,255,255,0.1);
  border-radius: var(--radius-sm);
  background:
    linear-gradient(rgba(255,255,255,0.045) 1px, transparent 1px),
    linear-gradient(90deg, rgba(255,255,255,0.045) 1px, transparent 1px),
    #0a0f16;
  background-size: 24px 24px;
}
.plane-label {
  position: absolute;
  top: 0.55rem;
  left: 0.65rem;
  color: #e5edf7;
  font-size: 0.78rem;
  font-weight: 800;
}
.plane-axis {
  position: absolute;
  display: grid;
  place-items: center;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: rgba(255,255,255,0.1);
  color: #fff;
  font-size: 0.72rem;
  font-weight: 800;
}
.plane-axis--a { right: 0.7rem; bottom: 0.65rem; }
.plane-axis--b { left: 0.65rem; top: 50%; transform: translateY(-50%); }
.plane-silhouette {
  position: absolute;
  left: 50%;
  top: 54%;
  width: 96px;
  height: 58px;
  border: 2px solid currentColor;
  border-radius: 50%;
  transform: translate(-50%, -50%) rotate(-14deg);
  box-shadow: inset 0 0 24px currentColor, 0 0 22px rgba(255,255,255,0.08);
  opacity: 0.82;
}
.rotation-plane--x { color: #ef4444; }
.rotation-plane--y { color: #22c55e; }
.rotation-plane--z { color: #3b82f6; }
.rotation-plane--y .plane-silhouette { transform: translate(-50%, -50%) rotate(18deg) scaleX(0.72); }
.rotation-plane--z .plane-silhouette { transform: translate(-50%, -50%) rotate(0deg) scaleX(1.12); }
.tool-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.45rem;
}
.tool-actions--light {
  grid-template-columns: 1fr 1fr;
}

.btn-action {
  display: inline-flex; align-items: center; gap: 0.35rem;
  justify-content: center;
  padding: 0.4rem 0.7rem; cursor: pointer;
  background: #151f2a; color: #e7edf5;
  border: 1px solid rgba(255,255,255,0.1); border-radius: var(--radius-sm);
  font: inherit; font-size: 0.85rem;
}
.btn-action:hover { transform: translateY(-1px); }
.btn-action:disabled {
  cursor: not-allowed;
  opacity: 0.55;
  transform: none;
}
.btn-add { background: var(--c-primary); color: #fff; border-color: var(--c-primary); }
.btn-add:hover { filter: brightness(1.1); }
.btn-delete { background: #4a1c1c; color: #ffdcdc; border-color: #6a2828; }
.btn-delete:hover { background: #5a2222; }

.small { font-size: 0.85rem; }

@media (max-width: 680px) {
  .lab-layout { grid-template-columns: 1fr; }
  .preview-strip {
    align-items: stretch;
    flex-direction: column;
  }
  .preview-actions {
    justify-content: flex-start;
    flex-wrap: wrap;
  }
}

@media (max-width: 760px) {
  .lab-session--immersive {
    grid-template-columns: 1fr;
  }
  .lab-object-panel {
    position: absolute;
    inset: 0 auto 0 0;
    width: min(88vw, 330px);
  }
  .lab-topbar {
    left: min(90vw, 344px);
    flex-wrap: wrap;
  }
}
</style>
