<script setup lang="ts">
import { X } from '@lucide/vue'
import { onMounted, onUnmounted } from 'vue'

interface Props {
  open: boolean
  title?: string
}
const props = defineProps<Props>()
const emit = defineEmits<{ close: [] }>()

function onKey(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.open) emit('close')
}
onMounted(() => window.addEventListener('keydown', onKey))
onUnmounted(() => window.removeEventListener('keydown', onKey))
</script>

<template>
  <Teleport to="body">
    <div v-if="props.open" class="modal" role="dialog" aria-modal="true">
      <div class="modal__backdrop" @click="emit('close')" />
      <div class="modal__panel">
        <header class="modal__head">
          <h3>{{ props.title }}</h3>
          <button class="modal__close" :aria-label="$t('common.close')" @click="emit('close')">
            <X :size="18" aria-hidden="true" />
          </button>
        </header>
        <div class="modal__body">
          <slot />
        </div>
        <footer v-if="$slots.footer" class="modal__footer">
          <slot name="footer" />
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.modal { position: fixed; inset: 0; z-index: 1000; display: grid; place-items: center; }
.modal__backdrop { position: absolute; inset: 0; background: rgba(0,0,0,0.5); }
.modal__panel {
  position: relative;
  background: var(--c-surface);
  color: var(--c-text);
  border: 1px solid var(--c-border);
  border-radius: var(--radius-lg);
  width: min(640px, 92vw);
  max-height: 86vh;
  display: flex; flex-direction: column;
  box-shadow: 0 12px 40px rgba(0,0,0,0.25);
}
.modal__head { display: flex; justify-content: space-between; align-items: center; padding: 1rem 1.25rem; border-bottom: 1px solid var(--c-border); }
.modal__head h3 { margin: 0; font-size: 1.05rem; }
.modal__close { background: none; border: 0; font-size: 1.5rem; cursor: pointer; color: var(--c-text-muted); }
.modal__body { padding: 1.25rem; overflow: auto; }
.modal__footer { padding: 0.75rem 1.25rem; border-top: 1px solid var(--c-border); display: flex; gap: 0.5rem; justify-content: flex-end; }
</style>
