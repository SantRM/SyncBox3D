<script setup lang="ts">
interface Props {
  variant?: 'neutral' | 'success' | 'warning' | 'danger' | 'info'
  color?: string // override (hex). Cuando se pasa, ignora variant.
}
const props = withDefaults(defineProps<Props>(), { variant: 'neutral' })
</script>

<template>
  <span
    class="badge"
    :class="!props.color ? `badge--${props.variant}` : null"
    :style="props.color ? { background: props.color, color: '#fff' } : undefined"
  >
    <slot />
  </span>
</template>

<style scoped>
.badge {
  display: inline-flex; align-items: center;
  padding: 0.15rem 0.55rem;
  border-radius: 999px;
  font-size: 0.75rem; font-weight: 600;
  line-height: 1.4;
}
.badge--neutral { background: var(--c-surface-3); color: var(--c-text); }
.badge--success { background: #16a34a; color: #fff; }
.badge--warning { background: #ca8a04; color: #fff; }
.badge--danger  { background: var(--c-danger); color: #fff; }
.badge--info    { background: var(--c-primary); color: #fff; }
</style>
