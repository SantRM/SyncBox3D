<script setup lang="ts">
// Visor 3D multi-modelo para laboratorios (Nivel 2).
//
// A diferencia de Viewer3D.vue (mono-modelo), aquí:
//  - El padre pasa una lista reactiva de "modelos" (instancias). Sincronizamos
//    la escena añadiendo, removiendo o actualizando transforms.
//  - Cada instancia tiene un id propio y un url opcional. Si url es null,
//    montamos un placeholder (cubo gris) con el nombre como label.
//  - Doble click selecciona una instancia (Raycaster contra los root objects).
//    Al seleccionarla la rodeamos con BoxHelper amarillo + TransformControls.
//  - TransformControls expone modo `translate`, `rotate` o `scale`. Mientras se
//    arrastra, OrbitControls se desactiva para no pelear.
//  - Emitimos `transform-change` durante el drag (para preview) y
//    `transform-commit` al soltar (para persistir vía PATCH).
//
// El padre resuelve los object URLs vía modelStore antes de pasarlos. Si no
// existe un modelo disponible, queda placeholder. Limitamos cargas simultáneas
// para no saturar red/GPU en laboratorios con modelos pesados.

import {
  ACESFilmicToneMapping,
  AmbientLight,
  Box3,
  BoxGeometry,
  BoxHelper,
  Color,
  DirectionalLight,
  GridHelper,
  Group,
  Mesh,
  MeshBasicMaterial,
  MeshStandardMaterial,
  PerspectiveCamera,
  PMREMGenerator,
  Raycaster,
  Scene,
  SphereGeometry,
  SpotLight,
  SpotLightHelper,
  SRGBColorSpace,
  Vector2,
  Vector3,
  WebGLRenderer,
  type Material,
  type Object3D,
  type Texture,
  type WebGLRenderTarget,
} from 'three'
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js'
import { RoomEnvironment } from 'three/examples/jsm/environments/RoomEnvironment.js'
import { TransformControls } from 'three/examples/jsm/controls/TransformControls.js'
import { DRACOLoader } from 'three/examples/jsm/loaders/DRACOLoader.js'
import { GLTFLoader, type GLTF } from 'three/examples/jsm/loaders/GLTFLoader.js'
import { clone as cloneSkeleton } from 'three/examples/jsm/utils/SkeletonUtils.js'
import { onBeforeUnmount, onMounted, shallowRef, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { getAccessToken } from '@/services/api'

export interface LabModel {
  /** id de la instancia (escena_instancia.id) — clave estable */
  instanceId: string
  /** object URL del .glb, o null para placeholder */
  url: string | null
  /** etiqueta legible "Nombre — modelo escena N" */
  label: string
  /** motivo visible cuando se usa placeholder */
  loadError?: string | null
  transform: LabTransform
}

export interface LabTransform {
  x: number
  y: number
  z: number
  scale: number
  rotX: number
  rotY: number
  rotZ: number
}

export type TransformMode = 'translate' | 'rotate' | 'scale'
export type TransformAxis = 'x' | 'y' | 'z'

export interface LabVector {
  x: number
  y: number
  z: number
}

export interface LabSize extends LabVector {
  max: number
}

export interface LabLighting {
  enabled: boolean
  intensity: number
  color: string
  position: LabVector
  target: LabVector
  angle: number
  penumbra: number
  distance: number
  autoTarget: boolean
}

interface Props {
  models: LabModel[]
  background?: string
  showGrid?: boolean
  mode?: TransformMode
  activeAxis?: TransformAxis | null
  editable?: boolean
  lighting?: LabLighting
  lightSelected?: boolean
  showLightCone?: boolean
  /** instancia seleccionada controlada por el padre (opcional) */
  selectedId?: string | null
}
const props = withDefaults(defineProps<Props>(), {
  showGrid: true,
  mode: 'translate',
  activeAxis: null,
  editable: true,
  lightSelected: false,
  showLightCone: true,
  selectedId: null,
})
const { t } = useI18n()

const emit = defineEmits<{
  (e: 'transform-change', instanceId: string, t: LabTransform): void
  (e: 'transform-commit', instanceId: string, t: LabTransform): void
  (e: 'light-origin-change', position: LabVector): void
  (e: 'light-origin-commit', position: LabVector): void
  (e: 'light-select', selected: boolean): void
  (e: 'model-load-error', instanceId: string, message: string): void
  (e: 'select', instanceId: string | null): void
}>()

const root = ref<HTMLDivElement | null>(null)
const loading = ref(0) // # de modelos en curso

interface MinimapPoint {
  id: string
  label: string
  x: number
  y: number
  selected: boolean
  edge: boolean
}
const minimapPoints = ref<MinimapPoint[]>([])
const minimapHeading = ref(0)

const renderer = shallowRef<WebGLRenderer | null>(null)
const scene = shallowRef<Scene | null>(null)
const camera = shallowRef<PerspectiveCamera | null>(null)
const orbit = shallowRef<OrbitControls | null>(null)
const gizmo = shallowRef<TransformControls | null>(null)
const gizmoHelper = shallowRef<Object3D | null>(null)
const boxHelper = shallowRef<BoxHelper | null>(null)
const ambientLight = shallowRef<AmbientLight | null>(null)
const keyLight = shallowRef<DirectionalLight | null>(null)
const spotLight = shallowRef<SpotLight | null>(null)
const spotTarget = shallowRef<Object3D | null>(null)
const spotHelper = shallowRef<SpotLightHelper | null>(null)
const spotMarker = shallowRef<Mesh | null>(null)
const targetMarker = shallowRef<Mesh | null>(null)
const gridHelper = shallowRef<GridHelper | null>(null)
const lightOriginSelected = ref(false)

// Map instanceId -> root Object3D ya en la escena.
const sceneNodes = new Map<string, Object3D>()
interface ModelAssetEntry {
  template?: Object3D
  promise?: Promise<Object3D>
  refs: number
}
const modelAssetCache = new Map<string, ModelAssetEntry>()
let frameId = 0
let resizeObs: ResizeObserver | null = null
let isDisposed = false
let environmentTarget: WebGLRenderTarget | null = null
const MIN_SCALE = 0.05
let scaleDragStart = 1
let currentGridSize = 0
let currentGridDivisions = 0
const MODEL_LOAD_CONCURRENCY = 4
const LIGHT_ORIGIN_PICK = '__light_origin__'
const MINIMAP_RADIUS = 40
const MINIMAP_MIN_WORLD_RANGE = 6
let lastMinimapUpdate = 0

const HIGHLIGHT_COLOR = 0xffd000 // amarillo cálido
const DEFAULT_LIGHTING: LabLighting = {
  enabled: false,
  intensity: 12,
  color: '#fff4d6',
  position: { x: 4, y: 6, z: 4 },
  target: { x: 0, y: 0, z: 0 },
  angle: 0.55,
  penumbra: 0.35,
  distance: 30,
  autoTarget: false,
}
const ACTIVE_AMBIENT_INTENSITY = 0.16
const ACTIVE_KEY_INTENSITY = 0.08

async function runLimited<T>(items: T[], limit: number, fn: (item: T) => Promise<void>): Promise<void> {
  let next = 0
  const workers = Array.from({ length: Math.min(limit, items.length) }, async () => {
    while (next < items.length) {
      const item = items[next++]!
      await fn(item)
    }
  })
  await Promise.all(workers)
}

function disposeObject(obj: Object3D): void {
  obj.traverse((child) => {
    type DisposableMesh = Object3D & {
      geometry?: { dispose?: () => void }
      material?:
        | { dispose?: () => void; map?: { dispose?: () => void } }
        | Array<{ dispose?: () => void; map?: { dispose?: () => void } }>
    }
    const m = child as DisposableMesh
    m.geometry?.dispose?.()
    if (Array.isArray(m.material)) {
      for (const mat of m.material) {
        mat.map?.dispose?.()
        mat.dispose?.()
      }
    } else if (m.material) {
      m.material.map?.dispose?.()
      m.material.dispose?.()
    }
  })
}

function retainModelAsset(url: string): void {
  const entry = modelAssetCache.get(url)
  if (entry) entry.refs += 1
}

function releaseModelAsset(url: string): void {
  const entry = modelAssetCache.get(url)
  if (!entry) return
  entry.refs = Math.max(0, entry.refs - 1)
  if (entry.refs === 0 && entry.template && !entry.promise) {
    disposeObject(entry.template)
    modelAssetCache.delete(url)
  }
}

function disposeSceneNode(node: Object3D): void {
  const assetUrl = typeof node.userData.assetUrl === 'string' ? node.userData.assetUrl : null
  if (assetUrl) releaseModelAsset(assetUrl)
  else disposeObject(node)
}

function safeCleanup(fn: () => void): void {
  try {
    fn()
  } catch (err) {
    console.warn('LabViewer3D cleanup warning:', err)
  }
}

function applyTransform(obj: Object3D, t: LabTransform): void {
  obj.position.set(t.x, t.y, t.z)
  obj.scale.setScalar(t.scale)
  obj.rotation.set(t.rotX, t.rotY, t.rotZ)
}

function readTransform(obj: Object3D): LabTransform {
  return {
    x: obj.position.x,
    y: obj.position.y,
    z: obj.position.z,
    scale: obj.scale.x,
    rotX: obj.rotation.x,
    rotY: obj.rotation.y,
    rotZ: obj.rotation.z,
  }
}

function normalizeUniformScale(obj: Object3D): void {
  const values = [obj.scale.x, obj.scale.y, obj.scale.z]
  let next = obj.scale.x
  let maxDelta = -1
  for (const v of values) {
    const delta = Math.abs(v - scaleDragStart)
    if (delta > maxDelta) {
      maxDelta = delta
      next = v
    }
  }
  const scalar = Number.isFinite(next) ? Math.max(Math.abs(next), MIN_SCALE) : scaleDragStart
  obj.scale.setScalar(scalar)
}

function vectorToPlain(v: Vector3): LabVector {
  return { x: v.x, y: v.y, z: v.z }
}

function isLightOriginObject(obj: Object3D | null | undefined): boolean {
  return obj === spotMarker.value
}

function emitLightOrigin(eventName: 'light-origin-change' | 'light-origin-commit', obj: Object3D): void {
  const position = vectorToPlain(obj.position)
  if (eventName === 'light-origin-change') emit('light-origin-change', position)
  else emit('light-origin-commit', position)
}

function sizeToPlain(size: Vector3): LabSize {
  return {
    x: size.x,
    y: size.y,
    z: size.z,
    max: Math.max(size.x, size.y, size.z),
  }
}

function objectBox(instanceId: string | null | undefined): Box3 | null {
  if (!instanceId) return null
  const node = sceneNodes.get(instanceId)
  if (!node) return null
  const box = new Box3().setFromObject(node)
  return box.isEmpty() ? null : box
}

function objectSize(instanceId: string | null | undefined): LabSize | null {
  const box = objectBox(instanceId)
  if (!box) return null
  return sizeToPlain(box.getSize(new Vector3()))
}

function getObjectSize(instanceId: string): LabSize | null {
  return objectSize(instanceId)
}

function labBounds(): Box3 | null {
  const box = new Box3()
  let hasNodes = false
  for (const node of sceneNodes.values()) {
    const nodeBox = new Box3().setFromObject(node)
    if (nodeBox.isEmpty()) continue
    box.union(nodeBox)
    hasNodes = true
  }
  return hasNodes ? box : null
}

function getLargestObjectSize(): LabSize | null {
  let largest: LabSize | null = null
  for (const id of sceneNodes.keys()) {
    const size = objectSize(id)
    if (!size) continue
    if (!largest || size.max > largest.max) largest = size
  }
  return largest
}

function getLabCenter(): LabVector | null {
  const box = labBounds()
  return box ? vectorToPlain(box.getCenter(new Vector3())) : null
}

function updateMinimap(now = performance.now(), force = false): void {
  if (!force && now - lastMinimapUpdate < 80) return
  lastMinimapUpdate = now

  const cam = camera.value
  if (!cam || props.models.length === 0) {
    minimapPoints.value = []
    minimapHeading.value = 0
    return
  }

  const cameraPosition = cam.position
  const centers: Array<{ id: string; label: string; center: Vector3 }> = []
  let maxDistance = MINIMAP_MIN_WORLD_RANGE
  for (const model of props.models) {
    const node = sceneNodes.get(model.instanceId)
    const box = node ? new Box3().setFromObject(node) : null
    const center = box && !box.isEmpty()
      ? box.getCenter(new Vector3())
      : new Vector3(model.transform.x, model.transform.y, model.transform.z)
    centers.push({ id: model.instanceId, label: model.label, center })
    maxDistance = Math.max(
      maxDistance,
      Math.hypot(center.x - cameraPosition.x, center.z - cameraPosition.z),
    )
  }

  const worldRange = Math.max(MINIMAP_MIN_WORLD_RANGE, maxDistance * 1.15)
  minimapPoints.value = centers.map(({ id, label, center }) => {
    const rawX = ((center.x - cameraPosition.x) / worldRange) * MINIMAP_RADIUS
    const rawY = (-(center.z - cameraPosition.z) / worldRange) * MINIMAP_RADIUS
    const dist = Math.hypot(rawX, rawY)
    const edge = dist > MINIMAP_RADIUS
    const scale = edge ? MINIMAP_RADIUS / dist : 1
    return {
      id,
      label,
      x: 50 + rawX * scale,
      y: 50 + rawY * scale,
      selected: props.selectedId === id,
      edge,
    }
  })

  const direction = cam.getWorldDirection(new Vector3())
  if (Math.hypot(direction.x, direction.z) > 0.0001) {
    minimapHeading.value = Math.atan2(direction.x, -direction.z) * 180 / Math.PI
  }
}

function disposeGrid(): void {
  const grid = gridHelper.value
  if (!grid) return
  scene.value?.remove(grid)
  disposeObject(grid)
  gridHelper.value = null
  currentGridSize = 0
  currentGridDivisions = 0
}

function syncGrid(): void {
  const sc = scene.value
  if (!sc) return
  if (!props.showGrid) {
    disposeGrid()
    return
  }

  const bounds = labBounds()
  const size = bounds?.getSize(new Vector3()) ?? new Vector3(20, 0, 20)
  const center = bounds?.getCenter(new Vector3()) ?? new Vector3()
  const largest = getLargestObjectSize()
  const footprint = Math.max(size.x, size.z, largest?.max ?? 0, 10)
  const nextSize = Math.max(20, footprint * 1.35)
  const nextDivisions = Math.min(160, Math.max(20, Math.ceil(nextSize)))
  const shouldRecreate = !gridHelper.value
    || Math.abs(currentGridSize - nextSize) > 0.5
    || currentGridDivisions !== nextDivisions

  if (shouldRecreate) {
    disposeGrid()
    const grid = new GridHelper(nextSize, nextDivisions, 0x444444, 0x222222)
    gridHelper.value = grid
    currentGridSize = nextSize
    currentGridDivisions = nextDivisions
    sc.add(grid)
  }

  const y = bounds ? bounds.min.y - Math.max(0.01, size.y * 0.002) : 0
  gridHelper.value?.position.set(center.x, y, center.z)
}

function syncGizmoMode(): void {
  if (!gizmo.value) return
  if (lightOriginSelected.value) {
    gizmo.value.setMode('translate')
    gizmo.value.showX = true
    gizmo.value.showY = true
    gizmo.value.showZ = true
    return
  }
  const mode = props.mode ?? 'translate'
  gizmo.value.setMode(mode)
  const axis = mode === 'rotate' ? props.activeAxis : null
  gizmo.value.showX = !axis || axis === 'x'
  gizmo.value.showY = !axis || axis === 'y'
  gizmo.value.showZ = !axis || axis === 'z'
}

function makePlaceholder(label: string, reason?: string | null): Object3D {
  const geom = new BoxGeometry(1, 1, 1)
  const mat = new MeshStandardMaterial({ color: 0x6b7280, roughness: 0.8, metalness: 0.1 })
  const mesh = new Mesh(geom, mat)
  const group = new Group()
  group.add(mesh)
  group.userData.placeholder = true
  group.userData.label = label
  group.userData.loadError = reason ?? null
  return group
}

const draco = new DRACOLoader()
draco.setDecoderPath('/draco/')
const gltfLoader = new GLTFLoader()
gltfLoader.setDRACOLoader(draco)

function applyAuthHeaders(loader: GLTFLoader): void {
  const token = getAccessToken()
  loader.setRequestHeader(token ? { Authorization: `Bearer ${token}` } : {})
}

type MeshObject = Object3D & { material?: Material | Material[] }
type TextureSlotMaterial = Material & {
  map?: Texture | null
  emissiveMap?: Texture | null
  envMapIntensity?: number
  metalness?: number
}

function preserveMaterialColor(rootNode: Object3D): void {
  rootNode.traverse((child) => {
    const materials = (child as MeshObject).material
    if (!materials) return
    for (const mat of Array.isArray(materials) ? materials : [materials]) {
      const textured = mat as TextureSlotMaterial
      if (textured.map) textured.map.colorSpace = SRGBColorSpace
      if (textured.emissiveMap) textured.emissiveMap.colorSpace = SRGBColorSpace
      if (typeof textured.envMapIntensity === 'number') textured.envMapIntensity = 0.45
      if (textured.map && typeof textured.metalness === 'number' && textured.metalness > 0.8) {
        textured.metalness = 0.35
      }
      mat.needsUpdate = true
    }
  })
}

function describeModelLoadError(err: unknown): string {
  if (err instanceof Error && err.message) return `${t('modelUpload.error')} ${err.message}`
  if (typeof ProgressEvent !== 'undefined' && err instanceof ProgressEvent) {
    return t('modelUpload.error')
  }
  return t('modelUpload.error')
}

async function loadModelTemplate(url: string): Promise<Object3D> {
  const cached = modelAssetCache.get(url)
  if (cached?.template) return cached.template
  if (cached?.promise) return cached.promise

  const entry: ModelAssetEntry = { refs: 0 }
  const promise = (async () => {
    loading.value++
    try {
      applyAuthHeaders(gltfLoader)
      const gltf = await new Promise<GLTF>((resolve, reject) => {
        gltfLoader.load(url, resolve, undefined, reject)
      })
      preserveMaterialColor(gltf.scene)
      entry.template = gltf.scene
      return gltf.scene
    } finally {
      loading.value--
      entry.promise = undefined
      if (!entry.template && entry.refs === 0) modelAssetCache.delete(url)
    }
  })().catch((err) => {
    modelAssetCache.delete(url)
    throw err
  })

  entry.promise = promise
  modelAssetCache.set(url, entry)
  return promise
}

async function loadModelInto(model: LabModel): Promise<Object3D> {
  if (!model.url) return makePlaceholder(model.label, model.loadError ?? t('equipment.modelRequired'))
  try {
    const template = await loadModelTemplate(model.url)
    const node = cloneSkeleton(template)
    retainModelAsset(model.url)
    node.userData.assetUrl = model.url
    return node
  } catch (err) {
    const message = describeModelLoadError(err)
    console.warn(`No se pudo cargar el modelo "${model.label}":`, err)
    emit('model-load-error', model.instanceId, message)
    return makePlaceholder(model.label, message)
  }
}

async function ensureNode(model: LabModel): Promise<Object3D> {
  let node = sceneNodes.get(model.instanceId)
  if (!node) {
    node = await loadModelInto(model)
    if (isDisposed) {
      disposeSceneNode(node)
      return node
    }
    node.userData.instanceId = model.instanceId
    node.userData.sourceUrl = model.url ?? null
    sceneNodes.set(model.instanceId, node)
    scene.value?.add(node)
  }
  applyTransform(node, model.transform)
  return node
}

async function syncModels(): Promise<void> {
  if (!scene.value || isDisposed) return
  const incoming = new Set(props.models.map((m) => m.instanceId))
  // Eliminar los que ya no están.
  for (const [id, node] of sceneNodes) {
    if (!incoming.has(id)) {
      scene.value.remove(node)
      disposeSceneNode(node)
      sceneNodes.delete(id)
      if (props.selectedId === id) setSelection(null)
    }
  }
  // Crear / actualizar con concurrencia limitada para no saturar red/GPU.
  await runLimited(props.models, MODEL_LOAD_CONCURRENCY, async (m) => {
    await ensureNode(m)
  })
  if (isDisposed) return
  updateBoxHelper()
  syncGrid()
}

function updateBoxHelper(): void {
  if (!boxHelper.value) return
  if (lightOriginSelected.value) {
    boxHelper.value.visible = false
    return
  }
  const sel = props.selectedId ? sceneNodes.get(props.selectedId) : null
  if (sel) {
    boxHelper.value.setFromObject(sel)
    boxHelper.value.visible = true
  } else {
    boxHelper.value.visible = false
  }
}

function objectCenter(instanceId: string | null | undefined): Vector3 | null {
  const box = objectBox(instanceId)
  if (!box) return null
  return box.getCenter(new Vector3())
}

function getObjectCenter(instanceId: string): { x: number; y: number; z: number } | null {
  const center = objectCenter(instanceId)
  return center ? { x: center.x, y: center.y, z: center.z } : null
}

function normalizedLighting(): LabLighting {
  return props.lighting ?? DEFAULT_LIGHTING
}

function syncLighting(): void {
  const ambient = ambientLight.value
  const key = keyLight.value
  const light = spotLight.value
  const target = spotTarget.value
  const helper = spotHelper.value
  const marker = spotMarker.value
  const targetDot = targetMarker.value
  if (!ambient || !key || !light || !target || !helper || !marker || !targetDot) return

  const cfg = normalizedLighting()
  ambient.intensity = cfg.enabled ? ACTIVE_AMBIENT_INTENSITY : 0.6
  key.intensity = cfg.enabled ? ACTIVE_KEY_INTENSITY : 1.1

  light.visible = cfg.enabled
  helper.visible = cfg.enabled && props.showLightCone
  marker.visible = cfg.enabled
  targetDot.visible = cfg.enabled
  if (!cfg.enabled) {
    light.intensity = 0
    if (lightOriginSelected.value) {
      lightOriginSelected.value = false
      if (gizmo.value?.object === marker) gizmo.value.detach()
      if (gizmoHelper.value) gizmoHelper.value.visible = false
      updateBoxHelper()
      syncGizmoMode()
    }
    return
  }

  light.intensity = cfg.intensity
  light.color.set(cfg.color || DEFAULT_LIGHTING.color)
  light.angle = cfg.angle
  light.penumbra = cfg.penumbra
  light.distance = Math.max(0.1, cfg.distance)

  const autoCenter = cfg.autoTarget ? objectCenter(props.selectedId) : null
  const nextTarget = autoCenter ?? new Vector3(cfg.target.x, cfg.target.y, cfg.target.z)
  const draggingOrigin = gizmo.value?.dragging && gizmo.value.object === marker
  if (!draggingOrigin) marker.position.set(cfg.position.x, cfg.position.y, cfg.position.z)
  light.position.copy(marker.position)
  target.position.copy(nextTarget)
  targetDot.position.copy(nextTarget)
  marker.scale.setScalar(lightOriginSelected.value ? 1.45 : 1)
  ;(marker.material as MeshBasicMaterial).color.copy(light.color)
  helper.update()

  if (props.lightSelected && !lightOriginSelected.value) {
    setLightOriginSelection(true)
  }
}

function setLightOriginSelection(selected: boolean): void {
  const marker = spotMarker.value
  if (!selected) {
    if (!lightOriginSelected.value) return
    lightOriginSelected.value = false
    if (gizmo.value?.object === marker) gizmo.value.detach()
    if (gizmoHelper.value) gizmoHelper.value.visible = false
    updateBoxHelper()
    emit('light-select', false)
    syncGizmoMode()
    return
  }

  if (!marker || !normalizedLighting().enabled) return
  lightOriginSelected.value = true
  updateBoxHelper()
  if (gizmo.value && props.editable) {
    gizmo.value.attach(marker)
    if (gizmoHelper.value) gizmoHelper.value.visible = true
  } else if (gizmoHelper.value) {
    gizmoHelper.value.visible = false
  }
  emit('light-select', true)
  syncGizmoMode()
}

function setSelection(instanceId: string | null): void {
  if (lightOriginSelected.value && instanceId === null) {
    updateBoxHelper()
    emit('select', null)
    return
  }
  if (lightOriginSelected.value) setLightOriginSelection(false)
  if (gizmo.value && instanceId && props.editable) {
    const node = sceneNodes.get(instanceId)
    if (node) {
      gizmo.value.attach(node)
      if (gizmoHelper.value) gizmoHelper.value.visible = true
    }
  } else if (gizmo.value) {
    gizmo.value.detach()
    if (gizmoHelper.value) gizmoHelper.value.visible = false
  }
  updateBoxHelper()
  emit('select', instanceId)
}

// Raycaster para doble click.
const raycaster = new Raycaster()
const mouseNDC = new Vector2()

function pickAt(clientX: number, clientY: number): string | null {
  const el = root.value
  if (!el || !camera.value) return null
  const rect = el.getBoundingClientRect()
  mouseNDC.x = ((clientX - rect.left) / rect.width) * 2 - 1
  mouseNDC.y = -((clientY - rect.top) / rect.height) * 2 + 1
  raycaster.setFromCamera(mouseNDC, camera.value)
  const marker = spotMarker.value
  if (marker?.visible) {
    const lightHits = raycaster.intersectObject(marker, true)
    if (lightHits.length > 0) return LIGHT_ORIGIN_PICK
  }
  const targets = Array.from(sceneNodes.values())
  const hits = raycaster.intersectObjects(targets, true)
  for (const h of hits) {
    let cur: Object3D | null = h.object
    while (cur && cur.userData.instanceId === undefined) cur = cur.parent
    if (cur?.userData?.instanceId) return cur.userData.instanceId as string
  }
  return null
}

function onDblClick(ev: MouseEvent): void {
  const id = pickAt(ev.clientX, ev.clientY)
  if (id === LIGHT_ORIGIN_PICK) {
    setLightOriginSelection(!lightOriginSelected.value)
    return
  }
  if (!id && lightOriginSelected.value) {
    setLightOriginSelection(false)
  }
  setSelection(id && props.selectedId === id ? null : id)
}

function frameAll(): void {
  if (isDisposed) return
  if (!camera.value || !orbit.value) return
  const nodes = Array.from(sceneNodes.values())
  if (nodes.length === 0) {
    camera.value.position.set(4, 3, 4)
    orbit.value.target.set(0, 0, 0)
    orbit.value.update()
    updateMinimap(performance.now(), true)
    return
  }
  const box = new Box3()
  for (const n of nodes) box.expandByObject(n)
  const size = box.getSize(new Vector3())
  const center = box.getCenter(new Vector3())
  const maxDim = Math.max(size.x, size.y, size.z) || 1
  const fov = (camera.value.fov * Math.PI) / 180
  const dist = (maxDim / 2) / Math.tan(fov / 2) * 1.6
  camera.value.position.set(center.x + dist, center.y + dist * 0.7, center.z + dist)
  camera.value.near = Math.max(dist / 100, 0.01)
  camera.value.far = dist * 100
  camera.value.updateProjectionMatrix()
  orbit.value.target.copy(center)
  orbit.value.update()
  updateMinimap(performance.now(), true)
}

function init(): void {
  const el = root.value
  if (!el || isDisposed) return

  const sc = new Scene()
  sc.background = new Color(props.background ?? '#1e232b')

  const cam = new PerspectiveCamera(45, 1, 0.1, 1000)
  cam.position.set(4, 3, 4)

  const rend = new WebGLRenderer({ antialias: true, powerPreference: 'high-performance' })
  rend.outputColorSpace = SRGBColorSpace
  rend.toneMapping = ACESFilmicToneMapping
  rend.toneMappingExposure = 1
  rend.setPixelRatio(Math.min(window.devicePixelRatio, 2))
  el.appendChild(rend.domElement)

  const pmrem = new PMREMGenerator(rend)
  environmentTarget = pmrem.fromScene(new RoomEnvironment(), 0.04)
  pmrem.dispose()
  sc.environment = environmentTarget.texture
  sc.environmentIntensity = 0.55

  const orb = new OrbitControls(cam, rend.domElement)
  orb.enableDamping = true
  orb.dampingFactor = 0.08

  const ambient = new AmbientLight(0xffffff, 0.6)
  sc.add(ambient)
  const dir = new DirectionalLight(0xffffff, 1.1)
  dir.position.set(5, 10, 7.5)
  sc.add(dir)

  const target = new Group()
  target.position.set(0, 0, 0)
  sc.add(target)

  const spot = new SpotLight(0xfff4d6, 0, 30, 0.55, 0.35, 0)
  spot.position.set(4, 6, 4)
  spot.castShadow = false
  spot.decay = 0
  spot.target = target
  sc.add(spot)

  const spotCone = new SpotLightHelper(spot, 0xfff4d6)
  spotCone.visible = false
  sc.add(spotCone)

  const marker = new Mesh(
    new SphereGeometry(0.09, 16, 8),
    new MeshBasicMaterial({ color: 0xfff4d6 }),
  )
  marker.userData.lightOrigin = true
  marker.visible = false
  sc.add(marker)

  const targetDot = new Mesh(
    new SphereGeometry(0.075, 16, 8),
    new MeshBasicMaterial({ color: 0x38bdf8 }),
  )
  targetDot.visible = false
  sc.add(targetDot)

  // BoxHelper de selección (lo asociamos al dummy group; lo movemos en updateBoxHelper).
  const dummyForHelper = new Group()
  sc.add(dummyForHelper)
  const helper = new BoxHelper(dummyForHelper, HIGHLIGHT_COLOR)
  helper.visible = false
  ;(helper.material as { depthTest?: boolean; transparent?: boolean }).depthTest = false
  ;(helper.material as { transparent?: boolean }).transparent = true
  helper.renderOrder = 999
  sc.add(helper)

  // TransformControls. En three 0.169 no es un Object3D: añadimos getHelper().
  const tc = new TransformControls(cam, rend.domElement)
  tc.setSize(0.9)
  // Mientras se interactúa con el gizmo, deshabilitar OrbitControls.
  tc.addEventListener('dragging-changed', (ev: { value: unknown }) => {
    if (!props.editable) {
      orb.enabled = true
      tc.detach()
      return
    }
    const dragging = Boolean(ev.value)
    orb.enabled = !dragging
    if (dragging && tc.object) scaleDragStart = tc.object.scale.x
    if (!dragging) {
      // Commit al soltar.
      const obj = tc.object
      if (isLightOriginObject(obj)) {
        emitLightOrigin('light-origin-commit', obj)
      } else if (obj?.userData?.instanceId) {
        if (props.mode === 'scale') normalizeUniformScale(obj)
        emit('transform-commit', obj.userData.instanceId as string, readTransform(obj))
      }
    }
  })
  tc.addEventListener('objectChange', () => {
    if (!props.editable) return
    const obj = tc.object
    if (isLightOriginObject(obj)) {
      emitLightOrigin('light-origin-change', obj)
      syncLighting()
      return
    }
    if (!obj?.userData?.instanceId) return
    if (props.mode === 'scale') normalizeUniformScale(obj)
    updateBoxHelper()
    emit('transform-change', obj.userData.instanceId as string, readTransform(obj))
  })
  const tcHelper = tc.getHelper()
  tcHelper.visible = false
  sc.add(tcHelper)

  scene.value = sc
  camera.value = cam
  renderer.value = rend
  orbit.value = orb
  gizmo.value = tc
  gizmoHelper.value = tcHelper
  boxHelper.value = helper
  ambientLight.value = ambient
  keyLight.value = dir
  spotLight.value = spot
  spotTarget.value = target
  spotHelper.value = spotCone
  spotMarker.value = marker
  targetMarker.value = targetDot
  syncGizmoMode()
  syncLighting()

  rend.domElement.addEventListener('dblclick', onDblClick)

  resizeObs = new ResizeObserver(() => {
    const w = el.clientWidth, h = el.clientHeight
    if (w > 0 && h > 0) {
      rend.setSize(w, h, false)
      cam.aspect = w / h
      cam.updateProjectionMatrix()
    }
  })
  resizeObs.observe(el)

  const tick = (): void => {
    if (isDisposed) return
    frameId = requestAnimationFrame(tick)
    orb.update()
    updateMinimap()
    if (boxHelper.value?.visible) {
      const sel = props.selectedId ? sceneNodes.get(props.selectedId) : null
      if (sel) boxHelper.value.setFromObject(sel)
    }
    syncLighting()
    rend.render(sc, cam)
  }
  tick()
}

onMounted(async () => {
  init()
  await syncModels()
  frameAll()
})

// Sincronizar cambios externos en la lista.
watch(
  () => props.models.map((m) => m.instanceId).join('|'),
  () => { void syncModels() },
)
// Sincronizar transforms cuando cambian externamente (e.g., restore).
watch(
  () => props.models.map((m) => `${m.instanceId}:${m.transform.x},${m.transform.y},${m.transform.z},${m.transform.scale},${m.transform.rotX},${m.transform.rotY},${m.transform.rotZ}`).join('|'),
  () => {
    for (const m of props.models) {
      const node = sceneNodes.get(m.instanceId)
      if (!node) continue
      // No piso si el gizmo está actualmente arrastrando ESTE nodo.
      if (gizmo.value?.dragging && gizmo.value.object === node) continue
      applyTransform(node, m.transform)
    }
    updateBoxHelper()
  },
)
// Sincronizar URL cuando cambia (raro pero posible si se recarga blob).
watch(
  () => props.models.map((m) => `${m.instanceId}:${m.url ?? ''}`).join('|'),
  async (n, o) => {
    if (n === o) return
    // Quitar los que tienen otra url y recrearlos.
    for (const m of props.models) {
      const node = sceneNodes.get(m.instanceId)
      if (!node) continue
      const currentUrl = typeof node.userData.sourceUrl === 'string' ? node.userData.sourceUrl : null
      const nextUrl = m.url ?? null
      if (currentUrl !== nextUrl) {
        scene.value?.remove(node)
        disposeSceneNode(node)
        sceneNodes.delete(m.instanceId)
        await ensureNode(m)
      }
    }
  },
)
watch(
  () => [props.mode, props.activeAxis] as const,
  () => { syncGizmoMode() },
)
watch(
  () => props.editable,
  () => {
    if (props.lightSelected) setLightOriginSelection(true)
    else setSelection(props.selectedId)
  },
)
watch(
  () => props.selectedId,
  (id) => setSelection(id),
)
watch(
  () => props.lightSelected,
  (selected) => setLightOriginSelection(selected),
)
watch(
  () => JSON.stringify(props.lighting ?? DEFAULT_LIGHTING),
  () => {
    syncLighting()
    if (props.lightSelected) setLightOriginSelection(true)
  },
)

onBeforeUnmount(() => {
  isDisposed = true
  const rend = renderer.value
  const canvas = rend?.domElement ?? null
  const sc = scene.value

  if (frameId) cancelAnimationFrame(frameId)
  frameId = 0

  safeCleanup(() => { resizeObs?.disconnect() })
  resizeObs = null

  if (canvas) {
    safeCleanup(() => { canvas.removeEventListener('dblclick', onDblClick) })
  }

  const tc = gizmo.value
  if (tc) {
    safeCleanup(() => { tc.detach() })
    safeCleanup(() => { tc.dispose() })
  }

  if (gizmoHelper.value && sc) {
    safeCleanup(() => { sc.remove(gizmoHelper.value as Object3D) })
  }
  if (boxHelper.value && sc) {
    safeCleanup(() => { sc.remove(boxHelper.value as Object3D) })
  }
  if (spotHelper.value && sc) {
    safeCleanup(() => { sc.remove(spotHelper.value as Object3D) })
    safeCleanup(() => { spotHelper.value?.dispose() })
  }
  if (spotMarker.value && sc) {
    safeCleanup(() => { sc.remove(spotMarker.value as Object3D) })
    safeCleanup(() => { disposeObject(spotMarker.value as Object3D) })
  }
  if (targetMarker.value && sc) {
    safeCleanup(() => { sc.remove(targetMarker.value as Object3D) })
    safeCleanup(() => { disposeObject(targetMarker.value as Object3D) })
  }

  for (const node of sceneNodes.values()) safeCleanup(() => { disposeSceneNode(node) })
  sceneNodes.clear()
  for (const entry of modelAssetCache.values()) {
    if (entry.template) safeCleanup(() => { disposeObject(entry.template as Object3D) })
  }
  modelAssetCache.clear()

  safeCleanup(() => { orbit.value?.dispose() })
  safeCleanup(() => { draco.dispose() })
  safeCleanup(() => { rend?.dispose() })
  safeCleanup(() => { environmentTarget?.dispose() })
  environmentTarget = null

  if (canvas?.parentElement) {
    safeCleanup(() => { canvas.parentElement?.removeChild(canvas) })
  }

  renderer.value = null
  scene.value = null
  camera.value = null
  orbit.value = null
  gizmo.value = null
  gizmoHelper.value = null
  boxHelper.value = null
  ambientLight.value = null
  keyLight.value = null
  spotLight.value = null
  spotTarget.value = null
  spotHelper.value = null
  spotMarker.value = null
  targetMarker.value = null
})

defineExpose({ frameAll, getObjectCenter })
</script>

<template>
  <div class="lab-viewer">
    <div ref="root" class="lab-viewer__canvas" :aria-busy="loading > 0" />
    <div v-if="loading > 0" class="lab-viewer__overlay">
      <span class="loader"></span>
      <p>{{ $t('labs.viewer.loadingModels') }}</p>
    </div>
    <div class="lab-viewer__hint" v-if="props.models.length === 0">
      {{ $t('labs.detail.noObjects') }}
    </div>
    <div v-if="props.models.length > 0" class="lab-minimap" :aria-label="$t('labs.viewer.minimapAria')">
      <svg class="lab-minimap__svg" viewBox="0 0 100 100" role="img" aria-hidden="true">
        <circle class="lab-minimap__rim" cx="50" cy="50" r="48" />
        <circle class="lab-minimap__ring" cx="50" cy="50" r="30" />
        <line class="lab-minimap__axis" x1="50" y1="9" x2="50" y2="91" />
        <line class="lab-minimap__axis" x1="9" y1="50" x2="91" y2="50" />
        <circle
          v-for="point in minimapPoints"
          :key="point.id"
          class="lab-minimap__point"
          :class="{ 'lab-minimap__point--selected': point.selected, 'lab-minimap__point--edge': point.edge }"
          :cx="point.x"
          :cy="point.y"
          :r="point.selected ? 4.2 : 3"
        >
          <title>{{ point.label }}</title>
        </circle>
        <g class="lab-minimap__user" :transform="`rotate(${minimapHeading} 50 50)`">
          <path d="M50 37 L56 55 L50 51 L44 55 Z" />
        </g>
        <circle class="lab-minimap__center" cx="50" cy="50" r="3.2" />
      </svg>
    </div>
  </div>
</template>

<style scoped>
.lab-viewer {
  position: relative;
  width: 100%;
  height: 100%;
  background: #0e1116;
  border-radius: var(--radius-md);
  overflow: hidden;
  border: 1px solid var(--c-border);
}
.lab-viewer__canvas { position: absolute; inset: 0; }
.lab-minimap {
  position: absolute;
  right: 0.9rem;
  bottom: 0.9rem;
  z-index: 5;
  width: clamp(94px, 12vw, 128px);
  aspect-ratio: 1;
  border-radius: 50%;
  background:
    radial-gradient(circle at center, rgba(15, 23, 42, 0.72), rgba(15, 23, 42, 0.9)),
    linear-gradient(135deg, rgba(56, 189, 248, 0.24), rgba(148, 163, 184, 0.1));
  border: 1px solid rgba(226, 232, 240, 0.28);
  box-shadow: 0 16px 34px rgba(0, 0, 0, 0.32);
  overflow: hidden;
  pointer-events: none;
  backdrop-filter: blur(10px);
}
.lab-minimap__svg {
  display: block;
  width: 100%;
  height: 100%;
}
.lab-minimap__rim {
  fill: rgba(15, 23, 42, 0.62);
  stroke: rgba(226, 232, 240, 0.24);
  stroke-width: 1.2;
}
.lab-minimap__ring {
  fill: none;
  stroke: rgba(148, 163, 184, 0.24);
  stroke-width: 0.8;
}
.lab-minimap__axis {
  stroke: rgba(148, 163, 184, 0.2);
  stroke-width: 0.75;
}
.lab-minimap__point {
  fill: #38bdf8;
  stroke: rgba(15, 23, 42, 0.82);
  stroke-width: 1.2;
}
.lab-minimap__point--selected {
  fill: #facc15;
  stroke: rgba(255, 255, 255, 0.8);
  stroke-width: 1.4;
}
.lab-minimap__point--edge {
  fill: #fb7185;
}
.lab-minimap__user path {
  fill: rgba(248, 250, 252, 0.94);
  stroke: rgba(15, 23, 42, 0.82);
  stroke-width: 1.1;
  stroke-linejoin: round;
}
.lab-minimap__center {
  fill: #22c55e;
  stroke: rgba(240, 253, 244, 0.95);
  stroke-width: 1.2;
}
.lab-viewer__overlay {
  position: absolute;
  inset: 0;
  z-index: 4;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-direction: column;
  gap: 1rem;
  background:
    radial-gradient(circle at center, rgba(14,17,22,0.54), rgba(14,17,22,0.18) 52%, transparent 72%);
  color: #fff;
  pointer-events: none;
}
.lab-viewer__overlay p {
  margin: 0;
  padding: 0.45rem 0.7rem;
  border-radius: var(--radius-sm);
  background: rgba(8, 12, 18, 0.72);
  text-align: center;
  font-size: 0.95rem;
  font-weight: 800;
  letter-spacing: 0;
  backdrop-filter: blur(8px);
}
.loader {
  position: relative;
  transform: rotateZ(45deg);
  perspective: 1000px;
  border-radius: 50%;
  width: 48px;
  height: 48px;
  color: #fff;
}
.loader:before,
.loader:after {
  content: '';
  display: block;
  position: absolute;
  top: 0;
  left: 0;
  width: inherit;
  height: inherit;
  border-radius: 50%;
  transform: rotateX(70deg);
  animation: 1s spin linear infinite;
}
.loader:after {
  color: #FF3D00;
  transform: rotateY(70deg);
  animation-delay: .4s;
}

@keyframes rotate {
  0% {
    transform: translate(-50%, -50%) rotateZ(0deg);
  }
  100% {
    transform: translate(-50%, -50%) rotateZ(360deg);
  }
}

@keyframes rotateccw {
  0% {
    transform: translate(-50%, -50%) rotate(0deg);
  }
  100% {
    transform: translate(-50%, -50%) rotate(-360deg);
  }
}

@keyframes spin {
  0%,
  100% {
    box-shadow: .2em 0px 0 0px currentcolor;
  }
  12% {
    box-shadow: .2em .2em 0 0 currentcolor;
  }
  25% {
    box-shadow: 0 .2em 0 0px currentcolor;
  }
  37% {
    box-shadow: -.2em .2em 0 0 currentcolor;
  }
  50% {
    box-shadow: -.2em 0 0 0 currentcolor;
  }
  62% {
    box-shadow: -.2em -.2em 0 0 currentcolor;
  }
  75% {
    box-shadow: 0px -.2em 0 0 currentcolor;
  }
  87% {
    box-shadow: .2em -.2em 0 0 currentcolor;
  }
}
.lab-viewer__hint {
  position: absolute; inset: auto 0 1rem 0;
  z-index: 4;
  margin: 0 auto; max-width: 380px; text-align: center;
  background: rgba(14,17,22,0.75); color: #cfd6e1;
  padding: 0.5rem 0.8rem; border-radius: var(--radius-sm);
  font-size: 0.85rem; pointer-events: none;
}

@media (max-width: 680px) {
  .lab-minimap {
    right: 0.65rem;
    bottom: 5rem;
    width: 84px;
  }
}
</style>
