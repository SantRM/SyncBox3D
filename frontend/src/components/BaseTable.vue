<script setup lang="ts" generic="T extends Record<string, unknown>">
interface Column<R> {
  key: string
  label: string
  render?: (row: R) => string
  width?: string
}
interface Props {
  rows: T[]
  columns: Column<T>[]
  empty?: string
  rowKey?: (row: T) => string | number
}
const props = defineProps<Props>()

function cellValue(row: T, key: string): unknown {
  return key.split('.').reduce<unknown>(
    (acc, k) => (acc && typeof acc === 'object' ? (acc as Record<string, unknown>)[k] : undefined),
    row,
  )
}
function keyFor(row: T, idx: number): string | number {
  return props.rowKey ? props.rowKey(row) : idx
}
</script>

<template>
  <div class="tbl-wrap">
    <table class="tbl">
      <thead>
        <tr>
          <th
            v-for="c in props.columns"
            :key="c.key"
            :style="c.width ? { width: c.width } : undefined"
          >
            {{ c.label }}
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-if="!props.rows.length">
          <td :colspan="props.columns.length" class="tbl__empty">{{ props.empty || $t('common.none') }}</td>
        </tr>
        <tr v-for="(row, i) in props.rows" :key="keyFor(row, i)">
          <td v-for="c in props.columns" :key="c.key">
            <slot :name="`cell-${c.key}`" :row="row">
              {{ c.render ? c.render(row) : (cellValue(row, c.key) ?? '—') }}
            </slot>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.tbl-wrap { overflow-x: auto; border: 1px solid var(--c-border); border-radius: var(--radius-md); }
.tbl { width: 100%; border-collapse: collapse; font-size: 0.92rem; }
.tbl th, .tbl td { padding: 0.65rem 0.85rem; text-align: left; border-bottom: 1px solid var(--c-border); vertical-align: middle; }
.tbl th { background: var(--c-surface-2); font-weight: 600; }
.tbl tbody tr:last-child td { border-bottom: 0; }
.tbl tbody tr:hover { background: var(--c-surface-2); }
.tbl__empty { text-align: center; color: var(--c-text-muted); padding: 1.5rem; }
</style>
