<script setup lang="ts">
interface Props {
  modelValue: string | number | null
  type?: 'text' | 'email' | 'password' | 'number' | 'search'
  label?: string
  placeholder?: string
  required?: boolean
  autocomplete?: string
  error?: string | null
  id?: string
  min?: string | number
  max?: string | number
  step?: string | number
  minlength?: string | number
  maxlength?: string | number
  pattern?: string
  title?: string
  inputmode?: 'none' | 'text' | 'decimal' | 'numeric' | 'tel' | 'search' | 'email' | 'url'
}
const props = withDefaults(defineProps<Props>(), { type: 'text', required: false })
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const inputId = props.id ?? `inp-${Math.random().toString(36).slice(2, 9)}`
</script>

<template>
  <label class="field">
    <span v-if="props.label" class="field__label">
      {{ props.label }}
      <span v-if="props.required" aria-hidden="true" class="field__req">*</span>
    </span>
    <input
      :id="inputId"
      class="field__input"
      :class="{ 'field__input--err': props.error }"
      :type="props.type"
      :value="props.modelValue"
      :placeholder="props.placeholder"
      :required="props.required"
      :autocomplete="props.autocomplete"
      :min="props.min"
      :max="props.max"
      :step="props.step"
      :minlength="props.minlength"
      :maxlength="props.maxlength"
      :pattern="props.pattern"
      :title="props.title"
      :inputmode="props.inputmode"
      @input="emit('update:modelValue', ($event.target as HTMLInputElement).value)"
    />
    <span v-if="props.error" class="field__error" role="alert">{{ props.error }}</span>
  </label>
</template>

<style scoped>
.field { display: flex; flex-direction: column; gap: 0.35rem; }
.field__label { font-size: 0.875rem; font-weight: 600; color: var(--c-text); }
.field__req { color: var(--c-danger); margin-left: 0.15rem; }
.field__input {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 0.55rem 0.75rem;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  background: var(--c-surface);
  color: var(--c-text);
  font: inherit;
}
.field__input:focus { outline: 2px solid var(--c-focus); outline-offset: 1px; border-color: var(--c-primary); }
.field__input--err { border-color: var(--c-danger); }
.field__error { color: var(--c-danger); font-size: 0.8rem; }
</style>
