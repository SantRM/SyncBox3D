<script setup lang="ts">
import type { Nodo, NodoTipo } from '@/services/types'

interface TreeNode extends Nodo {
  children?: TreeNode[]
  loaded?: boolean
  expanded?: boolean
}

const props = defineProps<{
  node: TreeNode
  selectedId: string | null
  isSelectable: (n: TreeNode) => boolean
  showContextActions: boolean
  clickToNavigate?: boolean
}>()

const emit = defineEmits<{
  (e: 'toggle', n: TreeNode): void
  (e: 'select', n: TreeNode): void
  (e: 'navigate', n: TreeNode): void
  (e: 'addChild', n: TreeNode): void
  (e: 'edit', n: TreeNode): void
  (e: 'delete', n: TreeNode): void
}>()

function tipoLabel(t: NodoTipo): string {
  return { UBICACION: 'U', LABORATORIO: 'L', EQUIPO: 'E' }[t]
}

function canHaveChildren(t: NodoTipo): boolean {
  return t === 'UBICACION'
}

function onNameClick(): void {
  if (props.clickToNavigate) {
    emit('navigate', props.node)
    return
  }
  emit('select', props.node)
}
</script>

<template>
  <li class="item">
    <div class="row" :class="{ selected: selectedId === node.id }">
      <button
        v-if="canHaveChildren(node.tipo)"
        type="button"
        class="caret"
        :aria-label="node.expanded ? $t('locations.treeItem.collapse') : $t('locations.treeItem.expand')"
        @click="emit('toggle', node)"
      >
        {{ node.expanded ? '▾' : '▸' }}
      </button>
      <span v-else class="caret-empty"></span>

      <span class="badge" :class="`t-${node.tipo}`">{{ tipoLabel(node.tipo) }}</span>

      <button
        type="button"
        class="name"
        :class="{ selectable: clickToNavigate || isSelectable(node) }"
        :disabled="!clickToNavigate && !isSelectable(node)"
        :title="node.path"
        @click="onNameClick"
      >
        {{ node.nombre }}
      </button>

      <span v-if="showContextActions" class="actions">
        <button
          v-if="canHaveChildren(node.tipo)"
          type="button"
          class="action"
          :title="$t('locations.treeItem.addChild')"
          @click="emit('addChild', node)"
        >+</button>
        <button type="button" class="action" :title="$t('common.edit')" @click="emit('edit', node)">✎</button>
        <button type="button" class="action danger" :title="$t('common.delete')" @click="emit('delete', node)">✕</button>
      </span>
    </div>

    <ul v-if="node.expanded && node.children && node.children.length" class="children">
      <NodoTreeItem
        v-for="c in node.children"
        :key="c.id"
        :node="c"
        :selected-id="selectedId"
        :is-selectable="isSelectable"
        :show-context-actions="showContextActions"
        :click-to-navigate="clickToNavigate"
        @toggle="(n) => emit('toggle', n)"
        @select="(n) => emit('select', n)"
        @navigate="(n) => emit('navigate', n)"
        @add-child="(n) => emit('addChild', n)"
        @edit="(n) => emit('edit', n)"
        @delete="(n) => emit('delete', n)"
      />
    </ul>
    <p v-else-if="node.expanded && node.loaded" class="empty">{{ $t('common.empty') }}</p>
  </li>
</template>

<script lang="ts">
export default { name: 'NodoTreeItem' }
</script>

<style scoped>
.item { list-style: none; }
.row { display: flex; align-items: center; gap: 0.4rem; padding: 0.15rem 0.3rem; border-radius: 4px; }
.row.selected { background: var(--c-primary, #4f46e5); color: #fff; }
.row.selected .name { color: #fff; }
.caret, .caret-empty { width: 1.2rem; text-align: center; }
.caret { background: none; border: 0; cursor: pointer; color: inherit; }
.badge { font-size: 0.7rem; font-weight: 700; padding: 0.05rem 0.35rem; border-radius: 4px; color: #fff; }
.t-UBICACION   { background: #4f46e5; }
.t-LABORATORIO { background: #10b981; }
.t-EQUIPO      { background: #f59e0b; }
.name { background: none; border: 0; cursor: default; color: inherit; padding: 0.15rem 0.3rem; border-radius: 4px; text-align: left; flex: 1; }
.name.selectable { cursor: pointer; }
.name.selectable:hover { background: rgba(0,0,0,0.06); }
.actions { display: flex; gap: 0.2rem; opacity: 0.7; }
.action { background: none; border: 1px solid var(--c-border, #ccc); border-radius: 3px; cursor: pointer; padding: 0 0.35rem; font-size: 0.85rem; color: inherit; }
.action.danger { color: var(--c-danger, #dc2626); border-color: var(--c-danger, #dc2626); }
.children { list-style: none; padding-left: 1.2rem; margin: 0; }
.empty { padding-left: 2.5rem; color: var(--c-text-muted); font-style: italic; font-size: 0.85rem; }
</style>
