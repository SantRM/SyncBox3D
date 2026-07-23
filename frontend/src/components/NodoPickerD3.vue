<script setup lang="ts">
/**
 * NodoPickerD3
 * ------------
 * Selector jerárquico de nodos con dos vistas:
 *  1. "overview": árbol con sólo UBICACIONES (verde claro). Click en una → drill.
 *  2. "drill":    sub-árbol del nodo elegido como pseudo-raíz, mostrando también
 *                 LABORATORIO (morado claro) y EQUIPO (azul claro).
 *
 * En modo drill aparecen botones "Atrás" (vuelve a overview) y "Agregar aquí"
 * (selecciona el nodo activo como destino y emite `pick`).
 *
 * Props:
 *   - allowSelectableTypes: tipos de nodo que pueden ser elegidos como destino.
 *     Por defecto sólo UBICACION.
 *   - excludeId: cuando se mueve un nodo, su id (y todos sus descendientes
 *     quedan excluidos como destino, por anti-ciclo).
 *   - title: encabezado descriptivo.
 */

import * as d3 from 'd3'
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import BaseButton from '@/components/BaseButton.vue'
import { api } from '@/services/api'
import type { Nodo, NodoTipo } from '@/services/types'

interface Props {
  allowSelectableTypes?: NodoTipo[]
  excludeId?: string | null
  rootId?: string | null
  title?: string
}
const props = withDefaults(defineProps<Props>(), {
  allowSelectableTypes: () => ['UBICACION'],
  excludeId: null,
  rootId: null,
  title: '',
})
const { t } = useI18n()

const emit = defineEmits<{
  (e: 'pick', nodeId: string): void
  (e: 'cancel'): void
}>()

const COLORS: Record<NodoTipo, { fill: string; stroke: string; text: string }> = {
  UBICACION:   { fill: '#e0e7ff', stroke: '#4f46e5', text: '#312e81' },
  LABORATORIO: { fill: '#d1fae5', stroke: '#10b981', text: '#064e3b' },
  EQUIPO:      { fill: '#fef3c7', stroke: '#f59e0b', text: '#78350f' },
}

// --- estado ----------------------------------------------------------------

const allNodos = ref<Nodo[]>([])           // árbol completo cacheado
const loading = ref(false)
const errorMsg = ref<string | null>(null)

const view = ref<'overview' | 'drill'>('overview')
const drilledId = ref<string | null>(null)
const targetId = ref<string | null>(null)  // nodo destino actualmente resaltado

const svgRef = ref<SVGSVGElement | null>(null)
const wrapperRef = ref<HTMLDivElement | null>(null)

// --- carga -----------------------------------------------------------------

async function loadAll(): Promise<void> {
  loading.value = true
  errorMsg.value = null
  try {
    const allRoots = (await api.nodos.list()) ?? []
    // Si recibimos rootId, sólo cargamos el subárbol de esa raíz (las raíces son
    // independientes entre sí). Sin rootId, cargamos todas.
    const targetRoots = props.rootId
      ? allRoots.filter((r) => r.id === props.rootId)
      : allRoots
    if (targetRoots.length === 0) {
      allNodos.value = []
      return
    }
    const subtrees = await Promise.all(targetRoots.map((r) => api.nodos.subtree(r.id)))
    const flat = new Map<string, Nodo>()
    for (const arr of subtrees) for (const n of arr ?? []) flat.set(n.id, n)
    allNodos.value = [...flat.values()]
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('locations.picker.loadError')
  } finally {
    loading.value = false
  }
}

// --- excluidos (subárbol del nodo a mover) --------------------------------

const excludedIds = computed<Set<string>>(() => {
  const set = new Set<string>()
  if (!props.excludeId) return set
  const target = allNodos.value.find((n) => n.id === props.excludeId)
  if (!target) return set
  const prefix = target.path
  for (const n of allNodos.value) {
    if (n.path === prefix || n.path.startsWith(prefix + '.')) set.add(n.id)
  }
  return set
})

// --- estructura jerárquica ------------------------------------------------

interface HNode {
  id: string
  nombre: string
  tipo: NodoTipo
  children?: HNode[]
}

function buildHierarchy(filterFn: (n: Nodo) => boolean, rootFinder: () => Nodo | null): HNode | null {
  const list = allNodos.value.filter(filterFn)
  if (list.length === 0) return null
  const root = rootFinder()
  if (!root) return null
  const byParent = new Map<string | null, Nodo[]>()
  for (const n of list) {
    const pid = n.parent_id ?? null
    if (!byParent.has(pid)) byParent.set(pid, [])
    byParent.get(pid)!.push(n)
  }
  for (const arr of byParent.values()) arr.sort((a, b) => a.orden - b.orden || a.nombre.localeCompare(b.nombre))
  function build(n: Nodo): HNode {
    const kids = (byParent.get(n.id) ?? []).map(build)
    return { id: n.id, nombre: n.nombre, tipo: n.tipo, children: kids.length ? kids : undefined }
  }
  return build(root)
}

const overviewRoot = computed<HNode | null>(() =>
  buildHierarchy(
    (n) => n.tipo === 'UBICACION',
    () => allNodos.value.find((n) => n.parent_id == null && n.tipo === 'UBICACION') ?? null,
  ),
)

const drillRoot = computed<HNode | null>(() => {
  if (!drilledId.value) return null
  const drilled = allNodos.value.find((n) => n.id === drilledId.value)
  if (!drilled) return null
  const prefix = drilled.path
  return buildHierarchy(
    (n) => n.path === prefix || n.path.startsWith(prefix + '.'),
    () => drilled,
  )
})

const currentRoot = computed<HNode | null>(() =>
  view.value === 'overview' ? overviewRoot.value : drillRoot.value,
)

// --- D3 render -------------------------------------------------------------

// xMin/xMax del último render: necesarios para calcular el scrollLeft que
// centra cualquier nodo (cuyo x_tree conocemos) dentro del viewport.
let lastXMin = 0
let lastSvgWidth = 0

function render(): void {
  const svgEl = svgRef.value
  const wrap = wrapperRef.value
  if (!svgEl || !wrap || !currentRoot.value) return

  const data = currentRoot.value
  const root = d3.hierarchy<HNode>(data)

  // Layout vertical (raíz arriba). Tamaño dinámico según número de nodos.
  const nodeW = 150
  const nodeH = 70
  const layout = d3.tree<HNode>().nodeSize([nodeW, nodeH])
  const positioned = layout(root)

  // Calcular extents para tamaño total del SVG. Forzamos un ancho simétrico
  // alrededor de x=0 (la raíz del árbol actual) para que ésta quede centrada
  // visualmente, sin importar que el subárbol sea asimétrico.
  let xMin = Infinity, xMax = -Infinity, yMax = 0
  positioned.each((d) => {
    const x = d.x ?? 0
    const y = d.y ?? 0
    if (x < xMin) xMin = x
    if (x > xMax) xMax = x
    if (y > yMax) yMax = y
  })
  const margin = 90
  const half = Math.max(Math.abs(xMin), Math.abs(xMax))
  const width = Math.max(420, half * 2 + margin * 2)
  const height = Math.max(220, yMax + margin * 2 + 30)

  // viewBox simétrico: x=0 cae exactamente en el centro del SVG.
  const viewBoxX = -width / 2
  lastXMin = viewBoxX
  lastSvgWidth = width

  const svg = d3.select(svgEl)
  svg.selectAll('*').remove()
  svg.attr('width', width).attr('height', height).attr('viewBox', `${viewBoxX} ${-margin} ${width} ${height}`)

  const g = svg.append('g')

  // Links
  g.append('g')
    .attr('fill', 'none')
    .attr('stroke', '#94a3b8')
    .attr('stroke-width', 1.4)
    .selectAll('path')
    .data(positioned.links())
    .join('path')
    .attr('d', d3.linkVertical<d3.HierarchyPointLink<HNode>, d3.HierarchyPointNode<HNode>>()
      .x((d) => d.x)
      .y((d) => d.y))

  // Node groups
  const node = g.append('g')
    .selectAll('g')
    .data(positioned.descendants())
    .join('g')
    .attr('transform', (d) => `translate(${d.x},${d.y})`)
    .style('cursor', (d) => isClickable(d.data) ? 'pointer' : 'not-allowed')
    .on('click', (_event, d) => onNodeClick(d.data))

  // Rect background
  const rectW = 130
  const rectH = 36
  node.append('rect')
    .attr('x', -rectW / 2)
    .attr('y', -rectH / 2)
    .attr('width', rectW)
    .attr('height', rectH)
    .attr('rx', 8)
    .attr('ry', 8)
    .attr('fill', (d) => excludedIds.value.has(d.data.id)
      ? '#e5e7eb'
      : COLORS[d.data.tipo].fill)
    .attr('stroke', (d) => d.data.id === targetId.value
      ? '#0f172a'
      : COLORS[d.data.tipo].stroke)
    .attr('stroke-width', (d) => d.data.id === targetId.value ? 3 : 1.5)
    .attr('opacity', (d) => excludedIds.value.has(d.data.id) ? 0.5 : 1)

  // Label
  node.append('text')
    .attr('text-anchor', 'middle')
    .attr('dominant-baseline', 'middle')
    .attr('font-size', '12px')
    .attr('font-weight', '600')
    .attr('fill', (d) => COLORS[d.data.tipo].text)
    .text((d) => trim(d.data.nombre, 18))
    .append('title')
    .text((d) => d.data.nombre)

  // Tipo tag (encima del rect)
  node.append('text')
    .attr('text-anchor', 'middle')
    .attr('y', -rectH / 2 - 6)
    .attr('font-size', '9px')
    .attr('font-weight', '700')
    .attr('fill', (d) => COLORS[d.data.tipo].stroke)
    .text((d) => t(`nodeTypes.${d.data.tipo}`))
}

function trim(s: string, n: number): string {
  return s.length > n ? s.slice(0, n - 1) + '…' : s
}

function isClickable(n: HNode): boolean {
  if (excludedIds.value.has(n.id)) return false
  if (view.value === 'overview') {
    // En overview, sólo UBICACIONES (y todas drillables).
    return n.tipo === 'UBICACION'
  }
  // En drill, click en cualquier nodo selectable cambia el target.
  return props.allowSelectableTypes.includes(n.tipo)
}

function onNodeClick(n: HNode): void {
  if (!isClickable(n)) return
  if (view.value === 'overview') {
    drilledId.value = n.id
    targetId.value = props.allowSelectableTypes.includes(n.tipo) ? n.id : null
    view.value = 'drill'
  } else {
    targetId.value = n.id
  }
}

function back(): void {
  view.value = 'overview'
  drilledId.value = null
  targetId.value = null
}

function confirmPick(): void {
  if (!targetId.value) return
  emit('pick', targetId.value)
}

// --- watchers --------------------------------------------------------------

// Centra el scroll horizontal del viewport en el nodo cuyo x_tree se pasa
// (0 = raíz). Si el SVG cabe entero en el viewport, scrollLeft se queda en 0
// (la raíz ya está al centro gracias al viewBox simétrico).
function centerScrollOnTreeX(treeX: number): void {
  const wrap = wrapperRef.value
  if (!wrap || !lastSvgWidth) return
  // El viewBox empieza en lastXMin (= -width/2), así que un nodo con
  // coordenada de árbol treeX cae en SVG-x = treeX - lastXMin.
  const pixelX = treeX - lastXMin
  const target = pixelX - wrap.clientWidth / 2
  wrap.scrollLeft = Math.max(0, target)
}

async function rerenderAndCenter(): Promise<void> {
  render()
  // Esperar a que el SVG actualice su tamaño antes de fijar scrollLeft.
  await nextTick()
  // El nodo "foco" es siempre el root del árbol actual (x_tree = 0):
  //  - overview: raíz global
  //  - drill:    el nodo en el que entramos (su subárbol)
  centerScrollOnTreeX(0)
}

watch([currentRoot, targetId, excludedIds], () => { void rerenderAndCenter() }, { deep: true })

onMounted(async () => {
  await loadAll()
  await rerenderAndCenter()
})
</script>

<template>
  <div class="picker">
    <header class="picker__head">
      <h3>{{ props.title || $t('locations.picker.title') }}</h3>
      <p v-if="view === 'overview'" class="muted">
        {{ $t('locations.picker.hint') }}
      </p>
      <p v-else class="muted">
        {{ $t('locations.picker.validHint') }}
      </p>
    </header>

    <div class="legend">
      <span class="lg lg--ubi">{{ $t('nodeTypes.UBICACION') }}</span>
      <span class="lg lg--lab">{{ $t('nodeTypes.LABORATORIO') }}</span>
      <span class="lg lg--eq">{{ $t('nodeTypes.EQUIPO') }}</span>
    </div>

    <p v-if="errorMsg" class="err">{{ errorMsg }}</p>
    <p v-if="loading" class="muted">{{ $t('locations.picker.loadingTree') }}</p>

    <div ref="wrapperRef" class="picker__viewport">
      <svg ref="svgRef" role="img" :aria-label="$t('locations.picker.aria')" />
      <p v-if="!loading && !currentRoot" class="muted center">{{ $t('locations.picker.emptyVisible') }}</p>
    </div>

    <footer class="picker__foot">
      <BaseButton v-if="view === 'drill'" variant="ghost" @click="back">← {{ $t('locations.picker.back') }}</BaseButton>
      <BaseButton variant="ghost" @click="emit('cancel')">{{ $t('common.cancel') }}</BaseButton>
      <BaseButton
        v-if="view === 'drill'"
        variant="primary"
        :disabled="!targetId"
        @click="confirmPick"
      >
        {{ $t('locations.picker.addHere') }}
      </BaseButton>
    </footer>
  </div>
</template>

<style scoped>
.picker { display: flex; flex-direction: column; gap: 0.85rem; }
.picker__head h3 { margin: 0 0 0.25rem; font-size: 1rem; }
.muted { color: var(--c-text-muted); margin: 0; font-size: 0.85rem; }
.center { text-align: center; padding: 2rem; }
.err { color: var(--c-danger); margin: 0; }

.legend { display: flex; gap: 0.5rem; flex-wrap: wrap; }
.lg { font-size: 0.7rem; font-weight: 700; padding: 0.15rem 0.5rem; border-radius: 999px; border: 1px solid; }
.lg--ubi { background: #e0e7ff; color: #312e81; border-color: #4f46e5; }
.lg--lab { background: #d1fae5; color: #064e3b; border-color: #10b981; }
.lg--eq  { background: #fef3c7; color: #78350f; border-color: #f59e0b; }

.picker__viewport {
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  background: var(--c-surface-2);
  overflow: auto;
  max-height: 50vh;
  min-height: 240px;
  padding: 0.5rem;
}
.picker__viewport svg { display: block; }

.picker__foot { display: flex; gap: 0.5rem; justify-content: flex-end; padding-top: 0.25rem; }
</style>
