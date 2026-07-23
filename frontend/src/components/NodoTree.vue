<script setup lang="ts">
import { onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

import NodoTreeItem from './NodoTreeItem.vue'
import { api } from '@/services/api'
import type { Nodo, NodoTipo } from '@/services/types'

export interface TreeNode extends Nodo {
  children?: TreeNode[]
  loaded?: boolean
  expanded?: boolean
}

const props = defineProps<{
  selectable?: boolean
  filterTipos?: NodoTipo[]
  modelValue?: string | null
  refreshKey?: number
  showContextActions?: boolean
  rootId?: string | null
  clickToNavigate?: boolean
}>()
const { t } = useI18n()

const emit = defineEmits<{
  (e: 'update:modelValue', id: string | null): void
  (e: 'select', node: Nodo): void
  (e: 'navigate', node: Nodo): void
  (e: 'addChild', node: Nodo): void
  (e: 'edit', node: Nodo): void
  (e: 'delete', node: Nodo): void
}>()

const roots = ref<TreeNode[]>([])
const loading = ref(false)
const errorMsg = ref<string | null>(null)
const nodeCache = new Map<string, Nodo>()
const childrenCache = new Map<string, Nodo[]>()

function canHaveChildren(node: Nodo): boolean {
  return node.tipo === 'UBICACION'
}

async function getNode(id: string): Promise<Nodo> {
  const cached = nodeCache.get(id)
  if (cached) return cached
  const node = await api.nodos.get(id)
  nodeCache.set(node.id, node)
  return node
}

async function getChildren(id: string): Promise<Nodo[]> {
  const cached = childrenCache.get(id)
  if (cached) return cached
  const kids = (await api.nodos.children(id)) ?? []
  for (const child of kids) nodeCache.set(child.id, child)
  childrenCache.set(id, kids)
  return kids
}

function clearCache(): void {
  nodeCache.clear()
  childrenCache.clear()
}

async function loadRoots(): Promise<void> {
  loading.value = true
  errorMsg.value = null
  try {
    if (props.rootId) {
      // Modo "iniciar desde un nodo específico": cargamos sólo ese nodo como
      // raíz visual del árbol; sus hijos se expanden con lazy-load.
      const single = await getNode(props.rootId)
      const kids = canHaveChildren(single) ? await getChildren(single.id) : []
      roots.value = [{
        ...single,
        expanded: kids.length > 0,
        loaded: true,
        children: kids.map((n) => ({ ...n, expanded: false, loaded: !canHaveChildren(n) })),
      }]
    } else {
      const data = (await api.nodos.list()) ?? []
      for (const node of data) nodeCache.set(node.id, node)
      roots.value = data.map((n) => ({ ...n, expanded: false, loaded: !canHaveChildren(n) }))
    }
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('locations.loadNodesError')
  } finally {
    loading.value = false
  }
}

async function toggle(node: TreeNode): Promise<void> {
  if (!canHaveChildren(node)) return
  if (node.expanded) {
    node.expanded = false
    return
  }
  if (!node.loaded) {
    try {
      const kids = await getChildren(node.id)
      node.children = kids.map((n) => ({ ...n, expanded: false, loaded: !canHaveChildren(n) }))
      node.loaded = true
    } catch (e) {
      errorMsg.value = (e as { message?: string }).message ?? t('locations.loadChildrenError')
      return
    }
  }
  node.expanded = true
}

function isSelectable(node: TreeNode): boolean {
  if (!props.selectable) return false
  if (!props.filterTipos) return true
  return props.filterTipos.includes(node.tipo)
}

function onSelect(node: TreeNode): void {
  if (!isSelectable(node)) return
  emit('update:modelValue', node.id)
  emit('select', node)
}

defineExpose({ reload: loadRoots })

onMounted(loadRoots)
watch(() => props.refreshKey, () => {
  clearCache()
  void loadRoots()
})
watch(() => props.rootId, () => void loadRoots())
</script>

<template>
  <div class="tree">
    <p v-if="errorMsg" class="err">{{ errorMsg }}</p>
    <p v-if="loading" class="muted">{{ $t('common.loading') }}</p>
    <ul class="list">
      <NodoTreeItem
        v-for="r in roots"
        :key="r.id"
        :node="r"
        :selected-id="props.modelValue ?? null"
        :is-selectable="isSelectable"
        :show-context-actions="!!props.showContextActions"
        :click-to-navigate="!!props.clickToNavigate"
        @toggle="toggle"
        @select="onSelect"
        @navigate="(n) => emit('navigate', n)"
        @add-child="(n) => emit('addChild', n)"
        @edit="(n) => emit('edit', n)"
        @delete="(n) => emit('delete', n)"
      />
    </ul>
  </div>
</template>

<style scoped>
.tree { font-size: 0.95rem; }
.list { list-style: none; padding-left: 0; margin: 0; }
.err { color: var(--c-danger); }
.muted { color: var(--c-text-muted); }
</style>
