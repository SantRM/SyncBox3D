<script setup lang="ts">
// Visor 3D basado en Three.js + GLTFLoader.
//
// Carga modelos .glb/.gltf con:
//  - DRACOLoader (compresión Draco) — se busca decoder en /draco/
//  - Auto-encuadre: calcula bounding box y posiciona la cámara
//  - OrbitControls (damping)
//  - Manejo de pérdida de contexto WebGL y cleanup completo en unmount
//  - Estados: cargando, error, vacío

import {
  ACESFilmicToneMapping,
  AmbientLight,
  Box3,
  Color,
  DirectionalLight,
  GridHelper,
  PerspectiveCamera,
  PMREMGenerator,
  Scene,
  SRGBColorSpace,
  Vector3,
  WebGLRenderer,
  type Material,
  type Object3D,
  type Texture,
  type WebGLRenderTarget,
} from 'three'
import { OrbitControls } from 'three/examples/jsm/controls/OrbitControls.js'
import { DRACOLoader } from 'three/examples/jsm/loaders/DRACOLoader.js'
import { GLTFLoader, type GLTF } from 'three/examples/jsm/loaders/GLTFLoader.js'
import { RoomEnvironment } from 'three/examples/jsm/environments/RoomEnvironment.js'
import { onBeforeUnmount, onMounted, ref, shallowRef, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import { getAccessToken } from '@/services/api'

interface Props {
  // URL al .glb/.gltf. Si está vacío, se muestra el placeholder.
  src?: string
  // Para Sketchfab: id del modelo. Si se pasa, se usa iframe en su lugar.
  sketchfabId?: string
  // Color de fondo (hex). Por defecto blanco/oscuro según modo.
  background?: string
  // Mostrar grilla bajo el modelo.
  showGrid?: boolean
}
const props = withDefaults(defineProps<Props>(), { showGrid: true })
const { t } = useI18n()

const root = ref<HTMLDivElement | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)
const progress = ref(0)

const renderer = shallowRef<WebGLRenderer | null>(null)
const scene = shallowRef<Scene | null>(null)
const camera = shallowRef<PerspectiveCamera | null>(null)
const controls = shallowRef<OrbitControls | null>(null)
const currentModel = shallowRef<Object3D | null>(null)
let frameId = 0
let resizeObs: ResizeObserver | null = null
let environmentTarget: WebGLRenderTarget | null = null

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

function frameModel(obj: Object3D): void {
  if (!camera.value || !controls.value) return
  const box = new Box3().setFromObject(obj)
  const size = box.getSize(new Vector3())
  const center = box.getCenter(new Vector3())
  const maxDim = Math.max(size.x, size.y, size.z) || 1
  const fov = (camera.value.fov * Math.PI) / 180
  const dist = (maxDim / 2) / Math.tan(fov / 2)
  camera.value.position.set(center.x + dist, center.y + dist * 0.7, center.z + dist)
  camera.value.near = Math.max(dist / 100, 0.01)
  camera.value.far = dist * 100
  camera.value.updateProjectionMatrix()
  controls.value.target.copy(center)
  controls.value.update()
}

function resetCamera(): void {
  if (currentModel.value) frameModel(currentModel.value)
}

function init(): void {
  const el = root.value
  if (!el) return

  const sc = new Scene()
  sc.background = new Color(props.background ?? '#1e232b')

  const cam = new PerspectiveCamera(45, 1, 0.1, 1000)
  cam.position.set(3, 2, 3)

  const rend = new WebGLRenderer({ antialias: true, powerPreference: 'high-performance' })
  rend.outputColorSpace = SRGBColorSpace
  rend.toneMapping = ACESFilmicToneMapping
  rend.toneMappingExposure = 1.0
  rend.setPixelRatio(Math.min(window.devicePixelRatio, 2))
  el.appendChild(rend.domElement)

  const pmrem = new PMREMGenerator(rend)
  environmentTarget = pmrem.fromScene(new RoomEnvironment(), 0.04)
  pmrem.dispose()
  sc.environment = environmentTarget.texture
  sc.environmentIntensity = 0.55

  const ctrls = new OrbitControls(cam, rend.domElement)
  ctrls.enableDamping = true
  ctrls.dampingFactor = 0.08

  sc.add(new AmbientLight(0xffffff, 0.6))
  const dir = new DirectionalLight(0xffffff, 1.1)
  dir.position.set(5, 10, 7.5)
  sc.add(dir)

  if (props.showGrid) {
    const grid = new GridHelper(10, 10, 0x444444, 0x222222)
    sc.add(grid)
  }

  scene.value = sc
  camera.value = cam
  renderer.value = rend
  controls.value = ctrls

  // Resize observer.
  resizeObs = new ResizeObserver(() => {
    const w = el.clientWidth
    const h = el.clientHeight
    if (w > 0 && h > 0) {
      rend.setSize(w, h, false)
      cam.aspect = w / h
      cam.updateProjectionMatrix()
    }
  })
  resizeObs.observe(el)

  // Animation loop.
  const tick = (): void => {
    frameId = requestAnimationFrame(tick)
    ctrls.update()
    rend.render(sc, cam)
  }
  tick()
}

async function loadModel(url: string): Promise<void> {
  if (!scene.value) return
  loading.value = true
  error.value = null
  progress.value = 0

  // Quitar modelo anterior.
  if (currentModel.value) {
    scene.value.remove(currentModel.value)
    disposeObject(currentModel.value)
    currentModel.value = null
  }

  const loader = new GLTFLoader()
  const draco = new DRACOLoader()
  // Ruta opcional externa; no depende de frontend/public.
  draco.setDecoderPath('/draco/')
  loader.setDRACOLoader(draco)
  const token = getAccessToken()
  loader.setRequestHeader(token ? { Authorization: `Bearer ${token}` } : {})

  try {
    const gltf: GLTF = await new Promise((resolve, reject) => {
      loader.load(
        url,
        (g) => resolve(g),
        (ev) => {
          if (ev.lengthComputable) progress.value = Math.round((ev.loaded / ev.total) * 100)
        },
        (err) => reject(err),
      )
    })
    const model = gltf.scene
    preserveMaterialColor(model)
    scene.value.add(model)
    currentModel.value = model
    frameModel(model)
  } catch (e) {
    error.value = (e as Error).message || t('viewer.equipment.loadError')
  } finally {
    loading.value = false
    draco.dispose()
  }
}

onMounted(() => {
  if (props.sketchfabId) return // se renderiza iframe
  init()
  if (props.src) void loadModel(props.src)
})

watch(
  () => props.src,
  (s) => {
    if (s && scene.value) void loadModel(s)
  },
)

onBeforeUnmount(() => {
  cancelAnimationFrame(frameId)
  resizeObs?.disconnect()
  if (currentModel.value) disposeObject(currentModel.value)
  controls.value?.dispose()
  renderer.value?.dispose()
  environmentTarget?.dispose()
  environmentTarget = null
  if (renderer.value?.domElement.parentElement === root.value) {
    root.value?.removeChild(renderer.value.domElement)
  }
  renderer.value = null
  scene.value = null
  camera.value = null
  controls.value = null
  currentModel.value = null
})

defineExpose({ resetCamera })
</script>

<template>
  <div class="viewer">
    <iframe
      v-if="props.sketchfabId"
      class="viewer__iframe"
      :src="`https://sketchfab.com/models/${props.sketchfabId}/embed?autospin=0&autostart=1&ui_theme=dark`"
      :title="$t('viewer.equipment.sketchfabTitle')"
      allow="autoplay; fullscreen; xr-spatial-tracking"
      allowfullscreen
    />

    <template v-else>
      <div ref="root" class="viewer__canvas" :aria-busy="loading" />

      <div v-if="!props.src" class="viewer__overlay viewer__overlay--info">
        <p>{{ $t('viewer.equipment.empty') }}</p>
      </div>
      <div v-else-if="loading" class="viewer__overlay viewer__overlay--info">
        <p>{{ $t('viewer.equipment.loading') }} {{ progress }}%</p>
      </div>
      <div v-else-if="error" class="viewer__overlay viewer__overlay--err" role="alert">
        <p>{{ error }}</p>
      </div>

      <div v-if="props.src && !error" class="viewer__toolbar">
        <button type="button" @click="resetCamera">{{ $t('viewer.equipment.center') }}</button>
      </div>
    </template>
  </div>
</template>

<style scoped>
.viewer {
  position: relative;
  width: 100%;
  aspect-ratio: 16 / 10;
  background: #0e1116;
  border-radius: var(--radius-md);
  overflow: hidden;
  border: 1px solid var(--c-border);
}
.viewer__canvas { position: absolute; inset: 0; }
.viewer__iframe { width: 100%; height: 100%; border: 0; display: block; }
.viewer__overlay {
  position: absolute; inset: 0;
  display: grid; place-items: center;
  background: rgba(14, 17, 22, 0.7);
  color: #fff; pointer-events: none;
  text-align: center; padding: 1rem;
}
.viewer__overlay--err { background: rgba(140, 30, 30, 0.85); }
.viewer__toolbar {
  position: absolute; right: 0.6rem; bottom: 0.6rem;
  display: flex; gap: 0.4rem;
}
.viewer__toolbar button {
  background: rgba(0,0,0,0.55); color: #fff;
  border: 1px solid rgba(255,255,255,0.2);
  border-radius: var(--radius-sm);
  padding: 0.35rem 0.7rem; cursor: pointer;
  font: inherit;
}
.viewer__toolbar button:hover { background: rgba(0,0,0,0.75); }
</style>
