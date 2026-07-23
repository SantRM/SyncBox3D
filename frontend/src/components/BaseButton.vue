<script setup lang="ts">
interface Props {
  variant?: 'primary' | 'secondary' | 'danger' | 'ghost'
  type?: 'button' | 'submit' | 'reset'
  disabled?: boolean
  loading?: boolean
}
const props = withDefaults(defineProps<Props>(), {
  variant: 'primary',
  type: 'button',
  disabled: false,
  loading: false,
})
</script>

<template>
  <button
    :type="props.type"
    :disabled="props.disabled || props.loading"
    :class="['btn', `btn--${props.variant}`, { 'btn--loading': props.loading }]"
  >
    <span v-if="props.loading" class="btn__spinner" aria-hidden="true" />
    <slot />
  </button>
</template>

<style scoped>
.btn {
  display: inline-flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.55rem 1rem;
  border-radius: var(--radius-md);
  border: 1px solid transparent;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
  transition: background-color 120ms, border-color 120ms, color 120ms;
}
.btn:focus-visible {
  outline: 2px solid var(--c-focus);
  outline-offset: 2px;
}
.btn[disabled] { opacity: 0.6; cursor: not-allowed; }
.btn--primary { background: var(--c-primary); color: #fff; }
.btn--primary:hover:not([disabled]) { background: var(--c-primary-700); }
.btn--secondary { background: var(--c-surface-2); color: var(--c-text); border-color: var(--c-border); }
.btn--secondary:hover:not([disabled]) { background: var(--c-surface-3); }
.btn--danger { background: var(--c-danger); color: #fff; }
.btn--danger:hover:not([disabled]) { background: var(--c-danger-700); }
.btn--ghost { background: transparent; color: var(--c-text); }
.btn--ghost:hover:not([disabled]) { background: var(--c-surface-2); }
.btn__spinner {
  width: 0.85em; height: 0.85em;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: spin 700ms linear infinite;
}
@keyframes spin { to { transform: rotate(360deg); } }
</style>
