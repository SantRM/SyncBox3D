<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import {
  ChevronLeft, ChevronRight, Pencil, Plus, Move, Trash2,
  FolderTree, Settings, Layers, Building2, Box, ExternalLink,
} from '@lucide/vue'

import AppFooter from '@/components/AppFooter.vue'
import AppNavbar from '@/components/AppNavbar.vue'
import BaseButton from '@/components/BaseButton.vue'
import BaseInput from '@/components/BaseInput.vue'
import Modal from '@/components/Modal.vue'
import NodoPickerD3 from '@/components/NodoPickerD3.vue'
import NodoTree from '@/components/NodoTree.vue'
import { api } from '@/services/api'
import type {
  Categoria, Equipo, Escena, EscenaDetail, EscenaInstancia, EstadoOperativo, Nodo, NodoTipo,
} from '@/services/types'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()

// ===========================================================================
// Estado general
// ===========================================================================

// Modos de la vista:
//   - 'list':   listado de raíces (cards) con acciones globales.
//   - 'detail': trabajo dentro de una raíz: pestañas Árbol/Gestión + breadcrumb.
const viewMode = ref<'list' | 'detail'>('list')
const selectedRoot = ref<Nodo | null>(null)

// Nodo actualmente enfocado dentro de la raíz (puede ser la propia raíz, o
// cualquier descendiente al que el usuario haya navegado en el árbol).
const currentNodeId = ref<string | null>(null)
const currentNode = ref<Nodo | null>(null)
const ancestors = ref<Nodo[]>([])

// Raiz visual del arbol y camino usado por los botones atras/adelante.
const treeRootId = ref<string | null>(null)
const treeNavPath = ref<string[]>([])
const treeNavIndex = ref(0)
const treeNodeCache = ref<Record<string, Nodo>>({})

// Pestaña activa en modo detail.
const panelTab = ref<'tree' | 'manage'>('tree')

// Listas asociadas al nodo actual (sólo se cargan en pestaña Gestión).
const directEquipos = ref<Equipo[]>([])
const directLabs = ref<Nodo[]>([])
const labSceneDetail = ref<EscenaDetail | null>(null)
const labAvailableEquipos = ref<Equipo[]>([])
const labLoading = ref(false)
const labError = ref<string | null>(null)
const labBusyEquipoId = ref<string | null>(null)
const labRemovingInstId = ref<string | null>(null)
const directChildEquipoNodes = ref<Nodo[]>([])  // hijos tipo EQUIPO (referencias en el árbol)

const refreshKey = ref(0)
const errorMsg = ref<string | null>(null)
const successMsg = ref<string | null>(null)

const roots = ref<Nodo[]>([])
const rootsLoading = ref(false)

// Catálogos cacheados para mostrar etiquetas amigables en la tabla de equipos.
const categorias = ref<Categoria[]>([])
const estados = ref<EstadoOperativo[]>([])
const escenas = ref<Escena[]>([])

const categoriaById = computed<Record<string, Categoria>>(() => {
  const m: Record<string, Categoria> = {}
  for (const c of categorias.value) m[c.id] = c
  return m
})
const estadoById = computed<Record<string, EstadoOperativo>>(() => {
  const m: Record<string, EstadoOperativo> = {}
  for (const e of estados.value) m[e.id] = e
  return m
})

async function loadCatalogs(): Promise<void> {
  try {
    const [c, s] = await Promise.all([api.categorias.list(true), api.estados.list()])
    categorias.value = c ?? []
    estados.value = s ?? []
  } catch {
    /* ignorar: la tabla mostrará IDs si falla */
  }
}

async function loadSceneLinks(): Promise<void> {
  try {
    escenas.value = (await api.escenas.list(false)) ?? []
  } catch {
    escenas.value = []
  }
}

async function loadRoots(): Promise<void> {
  rootsLoading.value = true
  try {
    roots.value = (await api.nodos.list()) ?? []
    if (selectedRoot.value) {
      const fresh = roots.value.find((r) => r.id === selectedRoot.value!.id) ?? null
      if (!fresh) {
        backToList()
      } else {
        selectedRoot.value = fresh
      }
    }
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('locations.loadRootsError')
  } finally {
    rootsLoading.value = false
  }
}

function flash(msg: string): void {
  successMsg.value = msg
  setTimeout(() => { if (successMsg.value === msg) successMsg.value = null }, 4000)
}

async function refreshAll(): Promise<void> {
  refreshKey.value++
  await loadRoots()
  if (currentNodeId.value) await loadCurrent()
}

const hasRoot = computed(() => roots.value.length > 0)
const treeVisualNode = computed(() => (treeRootId.value ? treeNodeCache.value[treeRootId.value] ?? null : null))
const canTreeBack = computed(() => treeNavIndex.value > 0)
const canTreeForward = computed(() => treeNavIndex.value < treeNavPath.value.length - 1)
const treeBackTarget = computed(() => nodeFromTreePath(treeNavIndex.value - 1))
const treeForwardTarget = computed(() => nodeFromTreePath(treeNavIndex.value + 1))

function nodeFromTreePath(index: number): Nodo | null {
  const id = treeNavPath.value[index]
  return id ? treeNodeCache.value[id] ?? null : null
}

// ---------------------------------------------------------------------------
// Navegación entre modos
// ---------------------------------------------------------------------------

function openRoot(r: Nodo): void {
  selectedRoot.value = r
  currentNodeId.value = r.id
  resetTreeNavigation(r)
  panelTab.value = 'tree'
  viewMode.value = 'detail'
  void loadCurrent()
}

function backToList(): void {
  viewMode.value = 'list'
  selectedRoot.value = null
  currentNodeId.value = null
  currentNode.value = null
  ancestors.value = []
  treeRootId.value = null
  treeNavPath.value = []
  treeNavIndex.value = 0
  treeNodeCache.value = {}
  directEquipos.value = []
  directLabs.value = []
  labSceneDetail.value = null
  labAvailableEquipos.value = []
  labError.value = null
  labBusyEquipoId.value = null
  labRemovingInstId.value = null
  directChildEquipoNodes.value = []
}

// El árbol o el breadcrumb piden enfocar otro nodo.
async function navigateToNode(id: string, switchToManage = true, syncTree = true): Promise<void> {
  const targetTab = switchToManage ? 'manage' : 'tree'
  const samePlace = currentNodeId.value === id && panelTab.value === targetTab
  currentNodeId.value = id
  if (switchToManage) panelTab.value = 'manage'
  if (samePlace && !syncTree) return
  const node = samePlace && currentNode.value?.id === id ? currentNode.value : await loadCurrent()
  if (syncTree && node) syncTreeNavigation(node)
}

// Carga del nodo actual + sus hijos / equipos directos.
async function loadCurrent(): Promise<Nodo | null> {
  if (!currentNodeId.value) return null
  const id = currentNodeId.value
  try {
    const [node, ancs, kids, eqs] = await Promise.all([
      api.nodos.get(id),
      api.nodos.ancestors(id),
      api.nodos.children(id),
      api.equipos.list({ nodo_id: id }),
    ])
    currentNode.value = node
    // ancestors devuelve la cadena hasta el nodo (incluido). Quitamos al actual
    // para que el breadcrumb sólo muestre los ascendientes.
    ancestors.value = (ancs ?? []).filter((n) => n.id !== id)
    const kidList = kids ?? []
    directLabs.value = kidList.filter((n) => n.tipo === 'LABORATORIO')
    directChildEquipoNodes.value = kidList.filter((n) => n.tipo === 'EQUIPO')
    if (node.tipo === 'EQUIPO') {
      directEquipos.value = (eqs ?? []).filter((e) => e.nodo_id === node.id)
    } else {
      const directEquipoNodeIds = new Set(directChildEquipoNodes.value.map((n) => n.id))
      directEquipos.value = (eqs ?? []).filter((e) => !!e.nodo_id && directEquipoNodeIds.has(e.nodo_id))
    }
    rememberTreeNodes([node, ...ancestors.value, ...kidList])
    return node
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('locations.loadNodeError')
    return null
  }
}

function rememberTreeNodes(nodes: Array<Nodo | null | undefined>): void {
  for (const n of nodes) {
    if (n) treeNodeCache.value[n.id] = n
  }
}

function resetTreeNavigation(root: Nodo): void {
  rememberTreeNodes([root])
  treeRootId.value = root.id
  treeNavPath.value = [root.id]
  treeNavIndex.value = 0
}

function syncTreeNavigation(node: Nodo): void {
  rememberTreeNodes([selectedRoot.value, ...ancestors.value, node])
  const ids = [...ancestors.value.map((n) => n.id), node.id]
  const selectedRootIndex = selectedRoot.value ? ids.indexOf(selectedRoot.value.id) : -1
  const scopedIds = selectedRootIndex >= 0 ? ids.slice(selectedRootIndex) : ids
  treeNavPath.value = scopedIds.length ? scopedIds : [node.id]
  treeNavIndex.value = treeNavPath.value.length - 1
  treeRootId.value = node.id
}

async function moveTreeWindow(delta: -1 | 1): Promise<void> {
  const nextIndex = treeNavIndex.value + delta
  if (nextIndex < 0 || nextIndex >= treeNavPath.value.length) return
  const id = treeNavPath.value[nextIndex]
  if (!id) return
  treeNavIndex.value = nextIndex
  treeRootId.value = id
  currentNodeId.value = id
  panelTab.value = 'tree'
  await loadCurrent()
}

watch(panelTab, (t) => {
  // Al volver al árbol, refrescamos para reflejar cambios en la jerarquía.
  if (t === 'tree') refreshKey.value++
  if (t === 'manage' && currentNode.value?.tipo === 'LABORATORIO') void loadLabManagement()
})

watch(currentNode, (node) => {
  if (panelTab.value === 'manage' && node?.tipo === 'LABORATORIO') {
    void loadLabManagement()
  } else if (node?.tipo !== 'LABORATORIO') {
    resetLabManagement()
  }
})

// ===========================================================================
// Crear (nodo)
//   - Modo 'root': desde el listado, sin padre.
//   - Modo 'child': desde el panel del nodo actual, padre = currentNodeId,
//     tipo elegido por el botón pulsado (UBICACION/LABORATORIO/EQUIPO).
//   - Modo 'free' (legacy): tree row "+" → abre picker.
// ===========================================================================

type CreateMode = 'root' | 'child' | 'free'
type CreateStep = 'form' | 'pick'

const createOpen = ref(false)
const createMode = ref<CreateMode>('child')
const createStep = ref<CreateStep>('form')
const createParentId = ref<string | null>(null)
const createTipo = ref<NodoTipo>('UBICACION')
const createNombre = ref('')
const createSlug = ref('')
const createOrden = ref(0)
const creating = ref(false)
const createErr = ref<string | null>(null)

function resetCreate(): void {
  createStep.value = 'form'
  createParentId.value = null
  createNombre.value = ''
  createSlug.value = ''
  createOrden.value = 0
  createErr.value = null
}

function openCreateRoot(): void {
  resetCreate()
  createMode.value = 'root'
  createTipo.value = 'UBICACION'
  createOpen.value = true
}

// Crear un hijo concreto bajo el nodo actual; si tipo='EQUIPO' navegamos al
// formulario completo (EquipoFormView) en vez de abrir el modal de nodo.
function openCreateChild(tipo: NodoTipo): void {
  if (!currentNodeId.value) return
  if (tipo === 'EQUIPO') {
    void router.push({
      name: 'equipo-nuevo',
      query: {
        parent_nodo_id: currentNodeId.value,
        return_root: selectedRoot.value?.id ?? '',
        return_node: currentNodeId.value,
      },
    })
    return
  }
  resetCreate()
  createMode.value = 'child'
  createParentId.value = currentNodeId.value
  createTipo.value = tipo
  createOpen.value = true
}

// Disparado desde una fila del árbol con el botón "+": el padre será ese nodo
// y el usuario decide el tipo en el modal.
function openCreateForNode(parent: Nodo): void {
  if (parent.tipo === 'LABORATORIO' || parent.tipo === 'EQUIPO') {
    errorMsg.value = t('locations.childForbidden')
    return
  }
  resetCreate()
  createMode.value = 'child'
  createParentId.value = parent.id
  createTipo.value = 'UBICACION'
  createOpen.value = true
}

// Tipos válidos para el selector dependiendo del padre actual (si lo hay).
const createTiposPermitidos = computed<NodoTipo[]>(() => {
  if (createMode.value === 'root') return ['UBICACION']
  // Si tenemos parentId conocido, restringimos por el tipo del padre.
  const parent = createMode.value === 'child' && createParentId.value
    ? findNodoLocal(createParentId.value)
    : null
  if (parent?.tipo === 'LABORATORIO') return []
  if (parent?.tipo === 'UBICACION') return ['UBICACION', 'LABORATORIO']
  return ['UBICACION', 'LABORATORIO']
})

function findNodoLocal(id: string): Nodo | null {
  if (currentNode.value?.id === id) return currentNode.value
  if (treeNodeCache.value[id]) return treeNodeCache.value[id]
  for (const n of [...ancestors.value, ...directLabs.value, ...directChildEquipoNodes.value]) {
    if (n.id === id) return n
  }
  return roots.value.find((r) => r.id === id) ?? null
}

const createTitle = computed(() => {
  if (createMode.value === 'root') return t('locations.newRoot')
  if (createTipo.value === 'LABORATORIO') return t('locations.newLab')
  if (createTipo.value === 'UBICACION') return t('locations.newSubLocation')
  return t('locations.newNode')
})

const createParentLabel = computed(() => {
  if (createMode.value === 'root') return t('locations.noParentRoot')
  const parent = createParentId.value ? findNodoLocal(createParentId.value) : null
  if (!parent) return t('locations.parentFromTree')
  return `${parent.nombre} / ${parent.slug}`
})

const createHelp = computed(() => {
  if (createMode.value === 'root') return t('locations.createRootHelp')
  if (createTipo.value === 'LABORATORIO') return t('locations.createLabHelp')
  return t('locations.createLocationHelp')
})

const createParentTypes = computed<NodoTipo[]>(() => {
  if (createTipo.value === 'UBICACION') return ['UBICACION']
  if (createTipo.value === 'LABORATORIO') return ['UBICACION']
  return ['UBICACION']
})

function nextStep(): void {
  if (!createNombre.value.trim()) {
    createErr.value = t('locations.nameRequired')
    return
  }
  if (createMode.value === 'root') {
    void submitCreate(null)
    return
  }
  if (createMode.value === 'child' && createParentId.value) {
    void submitCreate(createParentId.value)
    return
  }
  createErr.value = null
  createStep.value = 'pick'
}

async function submitCreate(parentId: string | null): Promise<void> {
  creating.value = true
  createErr.value = null
  try {
    const created = await api.nodos.create({
      tipo: createTipo.value,
      parent_id: parentId,
      nombre: createNombre.value.trim(),
      slug: createSlug.value.trim() || undefined,
      orden: createOrden.value || 0,
    })
    if (createTipo.value === 'LABORATORIO' && created?.id) {
      await api.escenas.create({
        nombre: created.nombre,
        descripcion: '',
        nodo_id: created.id,
      })
      await loadSceneLinks()
    }
    flash(t('locations.nodeCreated'))
    createOpen.value = false
    await refreshAll()
    // Si en modo detail acabamos de crear un sub-nodo, navegamos a él.
    if (viewMode.value === 'detail' && parentId === currentNodeId.value && created?.id) {
      navigateToNode(created.id, true)
    }
  } catch (e) {
    createErr.value = (e as { message?: string }).message ?? t('locations.createError')
    createStep.value = 'form'
  } finally {
    creating.value = false
  }
}

// ===========================================================================
// Editar nodo
// ===========================================================================

const editOpen = ref(false)
const editTarget = ref<Nodo | null>(null)
const editNombre = ref('')
const editSlug = ref('')
const editOrden = ref(0)
const editing = ref(false)
const editErr = ref<string | null>(null)

function openEdit(n: Nodo): void {
  editTarget.value = n
  editNombre.value = n.nombre
  editSlug.value = n.slug
  editOrden.value = n.orden
  editErr.value = null
  editOpen.value = true
}

async function submitEdit(): Promise<void> {
  if (!editTarget.value) return
  if (!editNombre.value.trim()) {
    editErr.value = t('locations.nameRequired')
    return
  }
  editing.value = true
  editErr.value = null
  try {
    await api.nodos.update(editTarget.value.id, {
      nombre: editNombre.value.trim(),
      slug: editSlug.value.trim() || undefined,
      orden: editOrden.value,
    })
    flash(t('locations.nodeUpdated'))
    editOpen.value = false
    await refreshAll()
  } catch (e) {
    editErr.value = (e as { message?: string }).message ?? t('locations.updateError')
  } finally {
    editing.value = false
  }
}

// ===========================================================================
// Mover nodo (D3 picker)
// ===========================================================================

const moveOpen = ref(false)
const moveTarget = ref<Nodo | null>(null)
const moveErr = ref<string | null>(null)
const moving = ref(false)

const movePermittedTypes = computed<NodoTipo[]>(() => {
  if (!moveTarget.value) return []
  if (moveTarget.value.tipo === 'UBICACION') return ['UBICACION']
  if (moveTarget.value.tipo === 'LABORATORIO') return ['UBICACION']
  return ['UBICACION']
})

function openMove(n: Nodo): void {
  if (!n.parent_id) {
    errorMsg.value = t('locations.rootCantMove')
    return
  }
  moveTarget.value = n
  moveErr.value = null
  moveOpen.value = true
}

async function submitMove(newParentId: string): Promise<void> {
  if (!moveTarget.value) return
  moving.value = true
  moveErr.value = null
  try {
    await api.nodos.move(moveTarget.value.id, newParentId)
    flash(t('locations.nodeMoved'))
    moveOpen.value = false
    await refreshAll()
  } catch (e) {
    moveErr.value = (e as { message?: string }).message ?? t('locations.moveError')
  } finally {
    moving.value = false
  }
}

// ===========================================================================
// Eliminar nodo
// ===========================================================================

const delOpen = ref(false)
const delTarget = ref<Nodo | null>(null)
const delConfirm = ref('')
const delPromote = ref(false)
const delReplacement = ref('')
const deleting = ref(false)
const delErr = ref<string | null>(null)

function openDelete(n: Nodo): void {
  delTarget.value = n
  delConfirm.value = ''
  delPromote.value = false
  delReplacement.value = ''
  delErr.value = null
  delOpen.value = true
}

async function submitDelete(): Promise<void> {
  if (!delTarget.value) return
  deleting.value = true
  delErr.value = null
  try {
    await api.nodos.delete(delTarget.value.id, {
      confirm: delConfirm.value || undefined,
      promote: delPromote.value || undefined,
      replacement_parent_id: delReplacement.value.trim() || undefined,
    })
    flash(t('locations.nodeDeleted'))
    delOpen.value = false
    // Si borramos el nodo actual, subimos al padre (o salimos al listado).
    if (delTarget.value.id === currentNodeId.value) {
      const upId = delTarget.value.parent_id
      if (upId) {
        currentNodeId.value = upId
        panelTab.value = 'manage'
      } else {
        backToList()
      }
    }
    await refreshAll()
  } catch (e) {
    delErr.value = (e as { message?: string }).message ?? t('locations.deleteError')
  } finally {
    deleting.value = false
  }
}

// ===========================================================================
// Acciones sobre equipos directos del nodo
// ===========================================================================

const eqDelOpen = ref(false)
const eqDelTarget = ref<Equipo | null>(null)
const eqDelConfirm = ref('')
const eqDeleting = ref(false)
const eqDelErr = ref<string | null>(null)

function openEquipoDelete(eq: Equipo): void {
  eqDelTarget.value = eq
  eqDelConfirm.value = ''
  eqDelErr.value = null
  eqDelOpen.value = true
}

async function submitEquipoDelete(): Promise<void> {
  if (!eqDelTarget.value) return
  if (eqDelConfirm.value.trim().toLowerCase() !== 'entiendo') {
    eqDelErr.value = t('locations.confirmExact')
    return
  }
  eqDeleting.value = true
  eqDelErr.value = null
  try {
    await api.equipos.delete(eqDelTarget.value.id)
    flash(t('locations.equipmentDeleted'))
    eqDelOpen.value = false
    await loadCurrent()
  } catch (e) {
    eqDelErr.value = (e as { message?: string }).message ?? t('locations.equipmentDeleteError')
  } finally {
    eqDeleting.value = false
  }
}

function gotoEquipo(eq: Equipo): void {
  void router.push({ name: 'equipo-detalle', params: { id: eq.id } })
}

// ===========================================================================
// Helpers de UI
// ===========================================================================

const tipoNodoActual = computed(() => currentNode.value?.tipo ?? null)
const currentLabScene = computed(() => {
  if (!currentNode.value || currentNode.value.tipo !== 'LABORATORIO') return null
  return escenas.value.find((e) => e.nodo_id === currentNode.value!.id) ?? null
})

const labUsedInstancias = computed<EscenaInstancia[]>(() => labSceneDetail.value?.instancias ?? [])
const labUsedCountByEquipo = computed<Record<string, number>>(() => {
  const out: Record<string, number> = {}
  for (const inst of labUsedInstancias.value) {
    if (!inst.equipo_origen_id) continue
    out[inst.equipo_origen_id] = (out[inst.equipo_origen_id] ?? 0) + 1
  }
  return out
})

function resetLabManagement(): void {
  labSceneDetail.value = null
  labAvailableEquipos.value = []
  labError.value = null
  labBusyEquipoId.value = null
  labRemovingInstId.value = null
}

async function loadLabManagement(): Promise<void> {
  if (!currentNode.value || currentNode.value.tipo !== 'LABORATORIO') {
    resetLabManagement()
    return
  }
  labLoading.value = true
  labError.value = null
  try {
    if (escenas.value.length === 0) await loadSceneLinks()
    const scene = currentLabScene.value
    if (!scene) {
      labSceneDetail.value = null
      labAvailableEquipos.value = []
      labError.value = t('locations.labNoScene')
      return
    }
    if (!currentNode.value.parent_id) {
      labSceneDetail.value = await api.escenas.get(scene.id)
      labAvailableEquipos.value = []
      return
    }

    const [detail, siblingNodes, parentEquipos] = await Promise.all([
      api.escenas.get(scene.id),
      api.nodos.children(currentNode.value.parent_id),
      api.equipos.list({ nodo_id: currentNode.value.parent_id, limit: 200 }),
    ])
    const siblingEquipoNodeIds = new Set(
      (siblingNodes ?? [])
        .filter((n) => n.tipo === 'EQUIPO')
        .map((n) => n.id),
    )
    labSceneDetail.value = detail
    labAvailableEquipos.value = (parentEquipos ?? []).filter((e) => !!e.nodo_id && siblingEquipoNodeIds.has(e.nodo_id))
  } catch (e) {
    labError.value = (e as { message?: string }).message ?? t('locations.labManagementLoadError')
  } finally {
    labLoading.value = false
  }
}

function nextLabPlacement(): Pick<EscenaInstancia, 'pos_x' | 'pos_y' | 'pos_z'> {
  const placed = labSceneDetail.value?.instancias ?? []
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
  return candidates.find((p) => !occupied.has(`${p.pos_x.toFixed(2)}:${p.pos_z.toFixed(2)}`)) ?? {
    pos_x: Math.cos(placed.length * 0.75) * spacing,
    pos_y: 0,
    pos_z: Math.sin(placed.length * 0.75) * spacing,
  }
}

async function addEquipoToLab(eq: Equipo): Promise<void> {
  const scene = labSceneDetail.value ?? currentLabScene.value
  if (!scene) return
  labBusyEquipoId.value = eq.id
  labError.value = null
  try {
    const inst = await api.escenas.addInstancia(scene.id, {
      equipo_id: eq.id,
      ...nextLabPlacement(),
    })
    if (labSceneDetail.value) {
      labSceneDetail.value.instancias = [...labSceneDetail.value.instancias, inst]
    } else {
      await loadLabManagement()
    }
    flash(t('locations.labEquipmentAdded'))
  } catch (e) {
    labError.value = (e as { message?: string }).message ?? t('locations.labEquipmentAddError')
  } finally {
    labBusyEquipoId.value = null
  }
}

async function removeEquipoFromLab(inst: EscenaInstancia): Promise<void> {
  const scene = labSceneDetail.value ?? currentLabScene.value
  if (!scene) return
  labRemovingInstId.value = inst.id
  labError.value = null
  try {
    await api.escenas.removeInstancia(scene.id, inst.id)
    if (labSceneDetail.value) {
      labSceneDetail.value.instancias = labSceneDetail.value.instancias.filter((i) => i.id !== inst.id)
    }
    flash(t('locations.labEquipmentRemoved'))
  } catch (e) {
    labError.value = (e as { message?: string }).message ?? t('locations.labEquipmentRemoveError')
  } finally {
    labRemovingInstId.value = null
  }
}

const labEditOpen = ref(false)
const labEditName = ref('')
const labEditDescription = ref('')
const labEditActive = ref(true)
const labEditBusy = ref(false)
const labEditErr = ref<string | null>(null)

function openLabSceneEdit(): void {
  const scene = labSceneDetail.value ?? currentLabScene.value
  if (!scene) return
  labEditName.value = scene.nombre
  labEditDescription.value = scene.descripcion
  labEditActive.value = scene.activo
  labEditErr.value = null
  labEditOpen.value = true
}

async function submitLabSceneEdit(): Promise<void> {
  const scene = labSceneDetail.value ?? currentLabScene.value
  if (!scene || !currentNode.value) return
  const name = labEditName.value.trim()
  if (!name) {
    labEditErr.value = t('locations.nameRequired')
    return
  }
  labEditBusy.value = true
  labEditErr.value = null
  try {
    await api.escenas.update(scene.id, {
      nombre: name,
      descripcion: labEditDescription.value.trim(),
      activo: labEditActive.value,
    })
    if (currentNode.value.nombre !== name) {
      await api.nodos.update(currentNode.value.id, { nombre: name })
    }
    labEditOpen.value = false
    await loadSceneLinks()
    await loadCurrent()
    await loadLabManagement()
    flash(t('locations.labUpdated'))
  } catch (e) {
    labEditErr.value = (e as { message?: string }).message ?? t('locations.labUpdateError')
  } finally {
    labEditBusy.value = false
  }
}

// Botones que se ofrecen para crear hijos según el tipo del nodo actual.
const childCreatableTypes = computed<NodoTipo[]>(() => {
  if (!tipoNodoActual.value) return []
  if (tipoNodoActual.value === 'UBICACION') return ['UBICACION', 'LABORATORIO', 'EQUIPO']
  if (tipoNodoActual.value === 'LABORATORIO') return []
  return []   // EQUIPO no puede tener hijos
})

const canMoveCurrent = computed(() => !!currentNode.value?.parent_id)

async function openCurrentLabScene(): Promise<void> {
  if (!currentNode.value || currentNode.value.tipo !== 'LABORATORIO') return
  if (escenas.value.length === 0) await loadSceneLinks()
  const scene = currentLabScene.value
  if (!scene) {
    errorMsg.value = t('locations.labNoScene')
    return
  }
  void router.push({
    name: 'laboratorio-detalle',
    params: { id: scene.id },
    query: {
      return_root: selectedRoot.value?.id ?? '',
      return_node: currentNode.value.id,
    },
  })
}

async function openManageTab(): Promise<void> {
  panelTab.value = 'manage'
  if (currentNode.value?.tipo === 'LABORATORIO') await loadLabManagement()
}

async function openLabNode(node: Nodo): Promise<void> {
  await navigateToNode(node.id, true, true)
  await loadLabManagement()
}

// ===========================================================================
// Mount: si vienen query params (vuelta desde EquipoForm), restaurar estado.
// ===========================================================================

onMounted(async () => {
  await Promise.all([loadRoots(), loadCatalogs(), loadSceneLinks()])
  const qRoot = typeof route.query.root === 'string' ? route.query.root : null
  const qNode = typeof route.query.node === 'string' ? route.query.node : null
  if (qRoot) {
    const r = roots.value.find((x) => x.id === qRoot)
    if (r) {
      selectedRoot.value = r
      currentNodeId.value = qNode || r.id
      panelTab.value = qNode && qNode !== r.id ? 'manage' : 'tree'
      viewMode.value = 'detail'
      const node = await loadCurrent()
      if (node) syncTreeNavigation(node)
      // Limpiamos los query params del URL sin recargar.
      void router.replace({ path: '/ubicaciones' })
    }
  }
})
</script>

<template>
  <div class="app-shell">
    <AppNavbar />
    <main class="page">
      <!-- =================== Modo: lista de raíces =================== -->
      <section v-if="viewMode === 'list'">
        <header class="head">
          <div>
            <h1>{{ $t('locations.title') }}</h1>
            <p class="muted">
              {{ $t('locations.listHint') }}
            </p>
          </div>
          <div class="head__actions">
            <BaseButton variant="primary" @click="openCreateRoot">
              <Plus :size="16" aria-hidden="true" /> {{ $t('locations.newRoot') }}
            </BaseButton>
          </div>
        </header>

        <p v-if="errorMsg" class="err" role="alert">{{ errorMsg }}</p>
        <p v-if="successMsg" class="ok" role="status">{{ successMsg }}</p>

        <p v-if="rootsLoading" class="muted">{{ $t('locations.loadingRoots') }}</p>
        <p v-else-if="!hasRoot" class="muted">
          {{ $t('locations.emptyRoots') }}
        </p>

        <ul v-else class="root-grid">
          <li v-for="r in roots" :key="r.id" class="root-card" @click="openRoot(r)">
            <div class="root-card__body">
              <span class="root-card__badge">{{ $t('locations.root') }}</span>
              <h3 class="root-card__name">{{ r.nombre }}</h3>
              <p class="root-card__slug">/{{ r.slug }}</p>
            </div>
            <div class="root-card__actions" @click.stop>
              <BaseButton variant="ghost" :title="$t('locations.editNamed', { name: r.nombre })" @click="openEdit(r)">
                <Pencil :size="14" aria-hidden="true" />
              </BaseButton>
              <BaseButton variant="ghost" :title="$t('locations.deleteNamed', { name: r.nombre })" @click="openDelete(r)">
                <Trash2 :size="14" aria-hidden="true" />
              </BaseButton>
              <BaseButton variant="primary" :title="$t('locations.openNamed', { name: r.nombre })" @click="openRoot(r)">
                {{ $t('common.open') }} <ChevronRight :size="14" aria-hidden="true" />
              </BaseButton>
            </div>
          </li>
        </ul>
      </section>

      <!-- =================== Modo: detalle de una raíz =================== -->
      <section v-else-if="viewMode === 'detail' && selectedRoot">
        <header class="head head--detail">
          <div class="head__main">
            <BaseButton variant="ghost" @click="backToList">
              <ChevronLeft :size="16" aria-hidden="true" /> {{ $t('locations.backToRoots') }}
            </BaseButton>

            <!-- Breadcrumb: raíz → ... → nodo actual -->
            <nav class="crumbs" :aria-label="$t('locations.breadcrumbAria')">
              <button
                type="button"
                class="crumb"
                :class="{ 'crumb--active': currentNodeId === selectedRoot.id }"
                @click="navigateToNode(selectedRoot.id, panelTab === 'manage')"
              >{{ selectedRoot.nombre }}</button>
              <template v-for="a in ancestors.filter((x) => x.id !== selectedRoot!.id)" :key="a.id">
                <span class="crumb__sep">/</span>
                <button
                  type="button"
                  class="crumb"
                  @click="navigateToNode(a.id, panelTab === 'manage')"
                >{{ a.nombre }}</button>
              </template>
              <template v-if="currentNode && currentNode.id !== selectedRoot.id">
                <span class="crumb__sep">/</span>
                <span class="crumb crumb--active">{{ currentNode.nombre }}</span>
              </template>
            </nav>
          </div>
        </header>

        <p v-if="errorMsg" class="err" role="alert">{{ errorMsg }}</p>
        <p v-if="successMsg" class="ok" role="status">{{ successMsg }}</p>

        <!-- Pestañas Árbol / Gestión -->
        <div class="tabs" role="tablist">
          <button
            type="button"
            class="tab"
            :class="{ 'tab--active': panelTab === 'tree' }"
            role="tab"
            :aria-selected="panelTab === 'tree'"
            @click="panelTab = 'tree'"
          >
            <FolderTree :size="14" aria-hidden="true" /> {{ $t('locations.tree') }}
          </button>
          <button
            type="button"
            class="tab"
            :class="{ 'tab--active': panelTab === 'manage' }"
            role="tab"
            :aria-selected="panelTab === 'manage'"
            @click="openManageTab"
          >
            <Settings :size="14" aria-hidden="true" />
            {{ $t('locations.management') }}<span v-if="currentNode"> {{ $t('locations.ofNode', { name: currentNode.nombre }) }}</span>
          </button>
        </div>

        <!-- Pestaña ÁRBOL ---------------------------------------------------- -->
        <div v-show="panelTab === 'tree'" class="tree-card">
          <p class="muted small">
            {{ $t('locations.clickHint') }}
          </p>
          <div class="tree-nav">
            <span v-if="treeVisualNode" class="tree-nav__current">
              {{ $t('locations.currentView') }}: <strong>{{ treeVisualNode.nombre }}</strong>
            </span>
            <div class="tree-nav__actions">
              <BaseButton
                variant="secondary"
                :disabled="!canTreeBack"
                :title="treeBackTarget ? $t('locations.viewParentNamed', { name: treeBackTarget.nombre }) : $t('locations.noPreviousParent')"
                :aria-label="$t('locations.previousParent')"
                @click="moveTreeWindow(-1)"
              >
                <ChevronLeft :size="16" aria-hidden="true" />
              </BaseButton>
              <BaseButton
                variant="secondary"
                :disabled="!canTreeForward"
                :title="treeForwardTarget ? $t('locations.returnToNamed', { name: treeForwardTarget.nombre }) : $t('locations.noNextChild')"
                :aria-label="$t('locations.nextChild')"
                @click="moveTreeWindow(1)"
              >
                <ChevronRight :size="16" aria-hidden="true" />
              </BaseButton>
            </div>
          </div>
          <NodoTree
            :refresh-key="refreshKey"
            :root-id="treeRootId"
            :model-value="currentNodeId"
            click-to-navigate
            show-context-actions
            @navigate="(n: Nodo) => navigateToNode(n.id, false, true)"
            @add-child="openCreateForNode"
            @edit="openEdit"
            @delete="openDelete"
          />
        </div>

        <!-- Pestaña GESTIÓN -------------------------------------------------- -->
        <div v-show="panelTab === 'manage'" class="manage">
          <p v-if="!currentNode" class="muted">{{ $t('locations.loadingNode') }}</p>
          <template v-else>
            <!-- Tarjeta del nodo + acciones -->
            <div class="node-card">
              <div class="node-card__head">
                <span class="badge" :class="`t-${currentNode.tipo}`">{{ currentNode.tipo }}</span>
                <h2 class="node-card__name">{{ currentNode.nombre }}</h2>
              </div>
              <p class="node-card__path">
                <code>{{ currentNode.path }}</code>
              </p>
              <div class="node-card__actions">
                <BaseButton variant="ghost" @click="openEdit(currentNode)">
                  <Pencil :size="14" aria-hidden="true" /> {{ $t('common.edit') }}
                </BaseButton>
                <BaseButton v-if="canMoveCurrent" variant="ghost" @click="openMove(currentNode)">
                  <Move :size="14" aria-hidden="true" /> {{ $t('common.move') }}
                </BaseButton>
                <BaseButton variant="danger" @click="openDelete(currentNode)">
                  <Trash2 :size="14" aria-hidden="true" /> {{ $t('common.delete') }}
                </BaseButton>
              </div>
            </div>

            <!-- Botones de creación de hijos -->
            <div v-if="childCreatableTypes.length" class="create-row">
              <span class="muted small">{{ $t('locations.createUnderNode') }}:</span>
              <BaseButton
                v-if="childCreatableTypes.includes('UBICACION')"
                variant="primary"
                @click="openCreateChild('UBICACION')"
              >
                <Building2 :size="14" aria-hidden="true" /> {{ $t('locations.subLocation') }}
              </BaseButton>
              <BaseButton
                v-if="childCreatableTypes.includes('LABORATORIO')"
                variant="primary"
                @click="openCreateChild('LABORATORIO')"
              >
                <Layers :size="14" aria-hidden="true" /> {{ $t('nodeTypes.LABORATORIO') }}
              </BaseButton>
              <BaseButton
                v-if="childCreatableTypes.includes('EQUIPO')"
                variant="primary"
                @click="openCreateChild('EQUIPO')"
              >
                <Box :size="14" aria-hidden="true" /> {{ $t('nodeTypes.EQUIPO') }}
              </BaseButton>
            </div>

            <!-- Gestion propia de LABORATORIO -->
            <section v-if="tipoNodoActual === 'LABORATORIO'" class="lab-manage block">
              <div class="lab-manage__head">
                <div>
                  <h3 class="block__title">
                    <Layers :size="14" aria-hidden="true" /> {{ $t('locations.labManagement') }}
                  </h3>
                  <p class="muted small" v-if="labSceneDetail?.descripcion">{{ labSceneDetail.descripcion }}</p>
                  <p class="muted small" v-else>{{ $t('locations.labManageHint') }}</p>
                </div>
                <div class="lab-manage__actions">
                  <BaseButton v-if="currentLabScene" variant="ghost" @click="openLabSceneEdit">
                    <Pencil :size="14" aria-hidden="true" /> {{ $t('locations.editLab') }}
                  </BaseButton>
                  <BaseButton v-if="currentLabScene" variant="primary" @click="openCurrentLabScene">
                    <ExternalLink :size="14" aria-hidden="true" /> {{ $t('locations.labDetail') }}
                  </BaseButton>
                </div>
              </div>

              <p v-if="labLoading" class="muted small">{{ $t('locations.loadingLab') }}</p>
              <p v-if="labError" class="err">{{ labError }}</p>

              <div v-if="currentLabScene" class="lab-summary">
                <span class="estado" :style="{ background: (labSceneDetail?.activo ?? currentLabScene.activo) ? '#10b981' : '#64748b' }">
                  {{ (labSceneDetail?.activo ?? currentLabScene.activo) ? $t('common.active') : $t('common.inactive') }}
                </span>
                <span>{{ $t('labs.detail.placedEquipmentCount', { count: labUsedInstancias.length }) }}</span>
                <span>{{ $t('locations.availableInLocation', { count: labAvailableEquipos.length }) }}</span>
              </div>

              <div v-if="currentLabScene" class="lab-manage__grid">
                <section class="lab-panel">
                  <h4>{{ $t('locations.availableEquipment') }}</h4>
                  <table v-if="labAvailableEquipos.length" class="eq-table">
                    <thead>
                      <tr>
                        <th>{{ $t('common.name') }}</th>
                        <th>{{ $t('common.status') }}</th>
                        <th class="num">{{ $t('common.actions') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="eq in labAvailableEquipos" :key="eq.id">
                        <td>
                          <button class="link" type="button" @click="gotoEquipo(eq)">{{ eq.nombre }}</button>
                          <small v-if="labUsedCountByEquipo[eq.id]" class="muted">
                            {{ $t('locations.inUseCount', { count: labUsedCountByEquipo[eq.id] }) }}
                          </small>
                        </td>
                        <td>
                          <span
                            class="estado"
                            :style="{ background: estadoById[eq.estado_id]?.color ?? '#999' }"
                          >{{ estadoById[eq.estado_id]?.nombre ?? '---' }}</span>
                        </td>
                        <td class="num">
                          <BaseButton
                            variant="primary"
                            :loading="labBusyEquipoId === eq.id"
                            @click="addEquipoToLab(eq)"
                          >
                            <Plus :size="12" aria-hidden="true" /> {{ $t('common.add') }}
                          </BaseButton>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                  <p v-else class="muted small">{{ $t('locations.noSiblingEquipment') }}</p>
                </section>

                <section class="lab-panel">
                  <h4>{{ $t('locations.usedEquipment') }}</h4>
                  <table v-if="labUsedInstancias.length" class="eq-table">
                    <thead>
                      <tr>
                        <th>{{ $t('alerts.equipment') }}</th>
                        <th>{{ $t('common.category') }}</th>
                        <th class="num">{{ $t('common.actions') }}</th>
                      </tr>
                    </thead>
                    <tbody>
                      <tr v-for="inst in labUsedInstancias" :key="inst.id">
                        <td>{{ inst.nombre_snapshot }}</td>
                        <td>{{ inst.categoria_snapshot || '---' }}</td>
                        <td class="num">
                          <BaseButton
                            variant="ghost"
                            :loading="labRemovingInstId === inst.id"
                            @click="removeEquipoFromLab(inst)"
                          >
                            <Trash2 :size="12" aria-hidden="true" /> {{ $t('common.remove') }}
                          </BaseButton>
                        </td>
                      </tr>
                    </tbody>
                  </table>
                  <p v-else class="muted small">{{ $t('locations.noLabEquipment') }}</p>
                </section>
              </div>
            </section>

            <!-- Laboratorios directos -->
            <section v-if="directLabs.length || tipoNodoActual === 'UBICACION'" class="block">
              <h3 class="block__title">
                <Layers :size="14" aria-hidden="true" /> {{ $t('locations.laboratories') }}
                <span class="muted small">({{ directLabs.length }})</span>
              </h3>
              <ul v-if="directLabs.length" class="lab-grid">
                <li v-for="l in directLabs" :key="l.id" class="lab-card" @click="openLabNode(l)">
                  <h4 class="lab-card__name">{{ l.nombre }}</h4>
                  <p class="lab-card__slug">/{{ l.slug }}</p>
                  <div class="lab-card__actions" @click.stop>
                    <BaseButton variant="ghost" @click="openEdit(l)"><Pencil :size="12" /></BaseButton>
                    <BaseButton variant="ghost" @click="openDelete(l)"><Trash2 :size="12" /></BaseButton>
                    <BaseButton variant="primary" @click="openLabNode(l)">{{ $t('locations.manage') }}</BaseButton>
                  </div>
                </li>
              </ul>
              <p v-else class="muted small">{{ $t('locations.noDirectLabs') }}</p>
            </section>

            <!-- Equipos directos -->
            <section v-if="tipoNodoActual !== 'LABORATORIO' && (directEquipos.length || tipoNodoActual !== 'EQUIPO')" class="block">
              <h3 class="block__title">
                <Box :size="14" aria-hidden="true" /> {{ $t('equipment.title') }}
                <span class="muted small">({{ directEquipos.length }})</span>
              </h3>
              <table v-if="directEquipos.length" class="eq-table">
                <thead>
                  <tr>
                    <th>{{ $t('common.name') }}</th>
                    <th>{{ $t('common.category') }}</th>
                    <th>{{ $t('common.status') }}</th>
                    <th>{{ $t('common.serial') }}</th>
                    <th class="num">{{ $t('common.actions') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr v-for="eq in directEquipos" :key="eq.id">
                    <td>
                      <button class="link" type="button" @click="gotoEquipo(eq)">{{ eq.nombre }}</button>
                    </td>
                    <td>{{ categoriaById[eq.categoria_id]?.nombre ?? '—' }}</td>
                    <td>
                      <span
                        class="estado"
                        :style="{ background: estadoById[eq.estado_id]?.color ?? '#999' }"
                      >{{ estadoById[eq.estado_id]?.nombre ?? '—' }}</span>
                    </td>
                    <td>{{ eq.serial || '—' }}</td>
                    <td class="num">
                      <BaseButton variant="ghost" :title="$t('locations.openEquipmentNamed', { name: eq.nombre })" @click="gotoEquipo(eq)">
                        <Pencil :size="12" />
                      </BaseButton>
                      <BaseButton variant="ghost" :title="$t('locations.deleteEquipmentNamed', { name: eq.nombre })" @click="openEquipoDelete(eq)">
                        <Trash2 :size="12" />
                      </BaseButton>
                    </td>
                  </tr>
                </tbody>
              </table>
              <p v-else class="muted small">{{ $t('locations.noNodeEquipment') }}</p>
            </section>

            <!-- Equipos como nodos hijos (referencias en árbol). Aviso si los hay
                 pero no aparecen como Equipos directos (caso poco común). -->
            <p v-if="directChildEquipoNodes.length && directEquipos.length === 0" class="muted small">
              {{ $t('locations.equipmentNodesWithoutCatalog', { count: directChildEquipoNodes.length }) }}
            </p>
          </template>
        </div>
      </section>
    </main>
    <AppFooter />

    <Modal
      :open="labEditOpen"
      :title="$t('locations.editLabTitle')"
      @close="labEditOpen = false"
    >
      <div class="form">
        <BaseInput v-model="labEditName" :label="$t('common.name')" :placeholder="$t('locations.labNamePlaceholder')" maxlength="150" required />
        <label class="select">
          <span>{{ $t('common.description') }}</span>
          <textarea v-model="labEditDescription" rows="3" maxlength="500" :placeholder="$t('locations.labDescriptionPlaceholder')" />
        </label>
        <label class="check">
          <input v-model="labEditActive" type="checkbox" />
          <span>{{ $t('common.active') }}</span>
        </label>
        <p v-if="labEditErr" class="err">{{ labEditErr }}</p>
      </div>
      <template #footer>
        <BaseButton variant="ghost" @click="labEditOpen = false">{{ $t('common.cancel') }}</BaseButton>
        <BaseButton variant="primary" :loading="labEditBusy" @click="submitLabSceneEdit">{{ $t('common.save') }}</BaseButton>
      </template>
    </Modal>

    <!-- ================= Crear nodo (paso 1: formulario) ================= -->
    <Modal
      :open="createOpen && createStep === 'form'"
      :title="createTitle"
      @close="createOpen = false"
    >
      <div class="form">
        <p class="muted small">{{ createHelp }}</p>
        <section class="create-context">
          <span>{{ $t('locations.willCreateUnder') }}</span>
          <strong>{{ createParentLabel }}</strong>
        </section>
        <label v-if="createMode !== 'root' && createTiposPermitidos.length > 1" class="select">
          <span>{{ $t('locations.type') }} *</span>
          <select v-model="createTipo">
            <option v-for="t in createTiposPermitidos" :key="t" :value="t">{{ $t(`nodeTypes.${t}`) }}</option>
          </select>
        </label>
        <BaseInput v-model="createNombre" :label="$t('common.name')" :placeholder="$t('locations.namePlaceholder')" maxlength="180" required />
        <BaseInput
          v-model="createSlug"
          :label="$t('locations.slugLabel')"
          placeholder="planta_norte"
          maxlength="180"
          pattern="[a-z0-9_]+"
          :title="$t('locations.slugTitle')"
        />
        <label class="select">
          <span>{{ $t('locations.order') }}</span>
          <input v-model.number="createOrden" type="number" step="1" />
        </label>
        <p v-if="createErr" class="err">{{ createErr }}</p>
      </div>
      <template #footer>
        <BaseButton variant="ghost" @click="createOpen = false">{{ $t('common.cancel') }}</BaseButton>
        <BaseButton variant="primary" :loading="creating" @click="nextStep">
          <template v-if="createMode === 'root'">{{ $t('locations.createRoot') }}</template>
          <template v-else-if="createMode === 'child'">{{ $t('common.create') }}</template>
          <template v-else>{{ $t('locations.nextPickParent') }}</template>
        </BaseButton>
      </template>
    </Modal>

    <!-- ============== Crear nodo (paso 2: picker D3, sólo modo libre) ============== -->
    <Modal
      :open="createOpen && createStep === 'pick'"
      :title="$t('locations.wherePlaceNode')"
      @close="createOpen = false"
    >
      <NodoPickerD3
        :allow-selectable-types="createParentTypes"
        :root-id="selectedRoot?.id ?? null"
        :title="$t('locations.pickParentFor', { name: createNombre, type: $t(`nodeTypes.${createTipo}`) })"
        @pick="submitCreate"
        @cancel="createStep = 'form'"
      />
      <p v-if="createErr" class="err">{{ createErr }}</p>
    </Modal>

    <!-- ================= Editar nodo ================= -->
    <Modal
      :open="editOpen"
      :title="editTarget ? $t('locations.editNamed', { name: editTarget.nombre }) : $t('common.edit')"
      @close="editOpen = false"
    >
      <div class="form">
        <BaseInput v-model="editNombre" :label="$t('common.name')" :placeholder="$t('locations.namePlaceholder')" maxlength="180" required />
        <BaseInput
          v-model="editSlug"
          label="Slug"
          placeholder="planta_norte"
          maxlength="180"
          pattern="[a-z0-9_]+"
          :title="$t('locations.slugTitle')"
        />
        <label class="select">
          <span>{{ $t('locations.order') }}</span>
          <input v-model.number="editOrden" type="number" step="1" />
        </label>
        <p v-if="editErr" class="err">{{ editErr }}</p>
      </div>
      <template #footer>
        <BaseButton variant="ghost" @click="editOpen = false">{{ $t('common.cancel') }}</BaseButton>
        <BaseButton v-if="editTarget?.parent_id" variant="ghost" @click="(editOpen = false, openMove(editTarget!))">
          <Move :size="14" aria-hidden="true" /> {{ $t('common.move') }}
        </BaseButton>
        <BaseButton variant="primary" :loading="editing" @click="submitEdit">{{ $t('common.save') }}</BaseButton>
      </template>
    </Modal>

    <!-- ================= Mover nodo (D3 picker) ================= -->
    <Modal
      :open="moveOpen"
      :title="moveTarget ? $t('locations.moveNamed', { name: moveTarget.nombre }) : $t('common.move')"
      @close="moveOpen = false"
    >
      <NodoPickerD3
        v-if="moveTarget"
        :allow-selectable-types="movePermittedTypes"
        :exclude-id="moveTarget.id"
        :root-id="selectedRoot?.id ?? null"
        :title="$t('locations.pickNewParentFor', { name: moveTarget.nombre, type: $t(`nodeTypes.${moveTarget.tipo}`) })"
        @pick="submitMove"
        @cancel="moveOpen = false"
      />
      <p v-if="moveErr" class="err">{{ moveErr }}</p>
      <p v-if="moving" class="muted">{{ $t('locations.moving') }}</p>
    </Modal>

    <!-- ================= Eliminar nodo ================= -->
    <Modal
      :open="delOpen"
      :title="delTarget ? $t('locations.deleteNamed', { name: delTarget.nombre }) : $t('common.delete')"
      @close="delOpen = false"
    >
      <div class="form">
        <p>
          {{ $t('locations.deleteWarning') }}
        </p>
        <BaseInput v-model="delConfirm" :label="$t('locations.confirmUnderstand')" :placeholder="$t('locations.confirmTextPlaceholder')" />
        <label class="check">
          <input v-model="delPromote" type="checkbox" />
          <span>{{ $t('locations.promoteChildren') }}</span>
        </label>
        <BaseInput v-model="delReplacement" :label="$t('locations.replacementParentId')" placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx" />
        <p v-if="delErr" class="err">{{ delErr }}</p>
      </div>
      <template #footer>
        <BaseButton variant="ghost" @click="delOpen = false">{{ $t('common.cancel') }}</BaseButton>
        <BaseButton variant="danger" :loading="deleting" @click="submitDelete">
          <Trash2 :size="14" aria-hidden="true" /> {{ $t('common.delete') }}
        </BaseButton>
      </template>
    </Modal>

    <!-- ================= Eliminar equipo ================= -->
    <Modal
      :open="eqDelOpen"
      :title="eqDelTarget ? $t('locations.deleteEquipmentNamed', { name: eqDelTarget.nombre }) : $t('locations.deleteEquipment')"
      @close="eqDelOpen = false"
    >
      <div class="form">
        <p>{{ $t('locations.deleteEquipmentWarning') }}</p>
        <BaseInput v-model="eqDelConfirm" :label="$t('locations.confirmUnderstand')" :placeholder="$t('locations.confirmTextPlaceholder')" />
        <p v-if="eqDelErr" class="err">{{ eqDelErr }}</p>
      </div>
      <template #footer>
        <BaseButton variant="ghost" @click="eqDelOpen = false">{{ $t('common.cancel') }}</BaseButton>
        <BaseButton variant="danger" :loading="eqDeleting" @click="submitEquipoDelete">
          <Trash2 :size="14" aria-hidden="true" /> {{ $t('common.delete') }}
        </BaseButton>
      </template>
    </Modal>
  </div>
</template>

<style scoped>
.head {
  display: flex; align-items: flex-start; justify-content: space-between;
  gap: 1rem; margin-bottom: 1rem;
}
.head--detail { flex-direction: column; gap: 0.5rem; }
.head__main { display: flex; flex-direction: column; gap: 0.5rem; width: 100%; }
.head h1 { margin: 0; }
.head__actions { display: flex; gap: 0.5rem; }
.muted { color: var(--c-text-muted); margin: 0.25rem 0 0; font-size: 0.9rem; }
.muted.small { font-size: 0.8rem; }
.center { text-align: center; padding: 1rem; }

.err { color: var(--c-danger); }
.ok  { color: #16a34a; }

.tree-card {
  background: var(--c-surface);
  border: 1px solid var(--c-border);
  border-radius: var(--radius-lg);
  padding: 1rem 1.25rem;
  margin-top: 1rem;
}
.tree-nav {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  flex-wrap: wrap;
  gap: 0.75rem;
  padding: 0.55rem 0 0.65rem;
  margin-bottom: 0.5rem;
  border-bottom: 1px solid var(--c-border);
}
.tree-nav__current {
  min-width: 0;
  color: var(--c-text-muted);
  font-size: 0.85rem;
}
.tree-nav__current strong {
  color: var(--c-text);
}
.tree-nav__actions {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  flex: 0 0 auto;
  order: -1;
}

/* === Breadcrumb ============================================== */
.crumbs {
  display: flex; flex-wrap: wrap; align-items: center; gap: 0.25rem;
  font-size: 0.95rem;
  background: var(--c-surface);
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  padding: 0.5rem 0.75rem;
}
.crumb {
  background: none; border: 0; padding: 0.15rem 0.4rem; cursor: pointer;
  color: var(--c-primary, #4f46e5); font: inherit; border-radius: 4px;
}
.crumb:hover { background: rgba(79, 70, 229, 0.08); }
.crumb--active { color: var(--c-text); font-weight: 700; cursor: default; }
.crumb--active:hover { background: transparent; }
.crumb__sep { color: var(--c-text-muted); padding: 0 0.1rem; }

/* === Tabs ===================================================== */
.tabs {
  display: flex; gap: 0.25rem; margin-top: 1rem; border-bottom: 1px solid var(--c-border);
}
.tab {
  display: inline-flex; align-items: center; gap: 0.4rem;
  padding: 0.55rem 0.9rem;
  background: transparent; border: 0; border-bottom: 3px solid transparent;
  cursor: pointer; color: var(--c-text-muted); font: inherit; font-weight: 600;
  border-radius: 4px 4px 0 0;
}
.tab:hover { color: var(--c-text); }
.tab--active { color: var(--c-primary, #4f46e5); border-bottom-color: var(--c-primary, #4f46e5); }

/* === Cards de raíces ====================================================== */
.root-grid {
  list-style: none; padding: 0; margin: 1rem 0 0;
  display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 1rem;
}
.root-card {
  display: flex; flex-direction: column; gap: 0.75rem;
  background: var(--c-surface); border: 1px solid var(--c-border);
  border-left: 4px solid #4f46e5; border-radius: var(--radius-lg);
  padding: 1rem 1.1rem; cursor: pointer;
  transition: transform 0.12s, box-shadow 0.12s, border-color 0.12s;
}
.root-card:hover {
  transform: translateY(-2px); box-shadow: 0 6px 18px rgba(0,0,0,0.08);
  border-color: #4f46e5;
}
.root-card__body { display: flex; flex-direction: column; gap: 0.25rem; }
.root-card__badge {
  align-self: flex-start; font-size: 0.65rem; font-weight: 800; letter-spacing: 0.06em;
  padding: 0.15rem 0.5rem; border-radius: 999px;
  background: #e0e7ff; color: #312e81; border: 1px solid #4f46e5;
}
.root-card__name { margin: 0.25rem 0 0; font-size: 1.1rem; }
.root-card__slug {
  margin: 0; font-size: 0.8rem; color: var(--c-text-muted);
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
}
.root-card__actions {
  display: flex; gap: 0.4rem; justify-content: flex-end; align-items: center;
  border-top: 1px solid var(--c-border); padding-top: 0.6rem;
}

/* === Panel Gestión ============================================ */
.manage { display: flex; flex-direction: column; gap: 1rem; padding-top: 1rem; }

.node-card {
  background: var(--c-surface); border: 1px solid var(--c-border);
  border-radius: var(--radius-lg); padding: 1rem 1.25rem;
  display: flex; flex-direction: column; gap: 0.5rem;
}
.node-card__head { display: flex; align-items: center; gap: 0.6rem; flex-wrap: wrap; }
.node-card__name { margin: 0; font-size: 1.25rem; }
.node-card__path { margin: 0; font-size: 0.85rem; color: var(--c-text-muted); }
.node-card__path code {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  background: var(--c-surface-2, transparent); padding: 0.1rem 0.35rem; border-radius: 4px;
}
.node-card__actions { display: flex; gap: 0.4rem; flex-wrap: wrap; }

.badge {
  font-size: 0.7rem; font-weight: 800; letter-spacing: 0.04em;
  padding: 0.2rem 0.5rem; border-radius: 999px; color: #fff;
}
.t-UBICACION   { background: #4f46e5; }
.t-LABORATORIO { background: #10b981; }
.t-EQUIPO      { background: #f59e0b; }

.create-row {
  display: flex; flex-wrap: wrap; align-items: center; gap: 0.5rem;
  background: var(--c-surface); border: 1px solid var(--c-border);
  border-radius: var(--radius-md); padding: 0.65rem 0.85rem;
}

.block { display: flex; flex-direction: column; gap: 0.5rem; }
.block__title {
  display: inline-flex; align-items: center; gap: 0.4rem; margin: 0; font-size: 1rem;
}

.lab-grid {
  list-style: none; padding: 0; margin: 0;
  display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 0.75rem;
}
.lab-card {
  background: var(--c-surface); border: 1px solid var(--c-border);
  border-left: 4px solid #10b981; border-radius: var(--radius-md);
  padding: 0.75rem 0.85rem; cursor: pointer;
  display: flex; flex-direction: column; gap: 0.25rem;
}
.lab-card:hover { box-shadow: 0 4px 12px rgba(16,185,129,0.12); border-color: #10b981; }
.lab-card__name { margin: 0; font-size: 1rem; }
.lab-card__slug { margin: 0; font-size: 0.75rem; color: var(--c-text-muted); font-family: ui-monospace, monospace; }
.lab-card__actions { display: flex; gap: 0.3rem; justify-content: flex-end; padding-top: 0.4rem; border-top: 1px solid var(--c-border); }

.lab-manage {
  background: var(--c-surface);
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  padding: 1rem;
}
.lab-manage__head,
.lab-manage__actions,
.lab-summary {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.lab-manage__head {
  justify-content: space-between;
}
.lab-manage__head p {
  margin: 0.3rem 0 0;
}
.lab-summary {
  color: var(--c-text-muted);
  font-size: 0.85rem;
}
.lab-manage__grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 1rem;
}
.lab-panel {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.lab-panel h4 {
  margin: 0;
  font-size: 0.95rem;
}
.lab-panel td small {
  display: block;
  margin-top: 0.15rem;
}

.eq-table {
  width: 100%; border-collapse: collapse;
  background: var(--c-surface); border: 1px solid var(--c-border); border-radius: var(--radius-md);
  overflow: hidden;
}
.eq-table th, .eq-table td {
  padding: 0.55rem 0.75rem; text-align: left; border-bottom: 1px solid var(--c-border);
  font-size: 0.9rem;
}
.eq-table th { background: var(--c-surface-2, transparent); font-weight: 700; }
.eq-table tr:last-child td { border-bottom: 0; }
.eq-table .num { text-align: right; }
.estado { display: inline-block; padding: 0.15rem 0.5rem; border-radius: 999px; color: #fff; font-size: 0.75rem; font-weight: 700; }
.link { background: none; border: 0; padding: 0; color: var(--c-primary, #4f46e5); cursor: pointer; font: inherit; text-decoration: underline; }

.form { display: flex; flex-direction: column; gap: 0.75rem; }
.create-context {
  display: grid;
  gap: 0.2rem;
  padding: 0.7rem 0.8rem;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  background: var(--c-surface-2);
}
.create-context span {
  color: var(--c-text-muted);
  font-size: 0.78rem;
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.select { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.875rem; font-weight: 600; }
.select select, .select input[type="number"], .select textarea {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 0.55rem 0.65rem;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  background: var(--c-surface);
  color: var(--c-text);
  font: inherit;
}
.check { display: inline-flex; align-items: center; gap: 0.5rem; font-size: 0.9rem; }

@media (max-width: 900px) {
  .lab-manage__grid { grid-template-columns: 1fr; }
}
</style>
