<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import { Pencil, Plus, Power, PowerOff } from '@lucide/vue'

import AppFooter from '@/components/AppFooter.vue'
import AppNavbar from '@/components/AppNavbar.vue'
import Badge from '@/components/Badge.vue'
import BaseButton from '@/components/BaseButton.vue'
import BaseInput from '@/components/BaseInput.vue'
import BaseTable from '@/components/BaseTable.vue'
import Modal from '@/components/Modal.vue'
import { api } from '@/services/api'
import type { Categoria } from '@/services/types'

const { t } = useI18n()

const cats = ref<Categoria[]>([])
const errorMsg = ref<string | null>(null)
const loading = ref(false)

const showCreate = ref(false)
const cName = ref('')
const cDesc = ref('')
const cBusy = ref(false)
const cErr = ref<string | null>(null)

const showEdit = ref(false)
const editTarget = ref<Categoria | null>(null)
const eName = ref('')
const eDesc = ref('')
const eActivo = ref(true)
const eBusy = ref(false)
const eErr = ref<string | null>(null)

async function load(): Promise<void> {
  loading.value = true
  errorMsg.value = null
  try {
    cats.value = await api.categorias.list(false)
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('categories.loadError')
  } finally {
    loading.value = false
  }
}

async function submit(): Promise<void> {
  if (!cName.value.trim()) {
    cErr.value = t('validation.required')
    return
  }
  cBusy.value = true
  cErr.value = null
  try {
    await api.categorias.create(cName.value.trim(), cDesc.value.trim())
    showCreate.value = false
    cName.value = ''; cDesc.value = ''
    await load()
  } catch (e) {
    cErr.value = (e as { message?: string }).message ?? t('categories.createError')
  } finally {
    cBusy.value = false
  }
}

function openEdit(c: Categoria): void {
  editTarget.value = c
  eName.value = c.nombre
  eDesc.value = c.descripcion ?? ''
  eActivo.value = c.activo
  eErr.value = null
  showEdit.value = true
}

async function submitEdit(): Promise<void> {
  if (!editTarget.value) return
  const trimmed = eName.value.trim()
  if (!trimmed) {
    eErr.value = t('validation.required')
    return
  }
  eBusy.value = true
  eErr.value = null
  try {
    const target = editTarget.value
    const patch: { nombre?: string; descripcion?: string; activo?: boolean } = {}
    if (trimmed !== target.nombre) patch.nombre = trimmed
    const desc = eDesc.value.trim()
    if (desc !== (target.descripcion ?? '')) patch.descripcion = desc
    if (eActivo.value !== target.activo) patch.activo = eActivo.value
    if (Object.keys(patch).length > 0) {
      await api.categorias.update(target.id, patch)
    }
    showEdit.value = false
    await load()
  } catch (e) {
    eErr.value = (e as { message?: string }).message ?? t('categories.updateError')
  } finally {
    eBusy.value = false
  }
}

async function toggleActivo(c: Categoria): Promise<void> {
  try {
    await api.categorias.update(c.id, { activo: !c.activo })
    await load()
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('categories.updateError')
  }
}

onMounted(load)

const cols = computed(() => [
  { key: 'nombre', label: t('common.name') },
  { key: 'descripcion', label: t('common.description') },
  { key: 'activo', label: t('common.status') },
  { key: 'acciones', label: '' },
])
</script>

<template>
  <div class="app-shell">
    <AppNavbar />
    <main class="page">
  <section>
    <header class="head">
      <h1>{{ $t('categories.title') }}</h1>
      <BaseButton @click="showCreate = true">
        <Plus :size="16" aria-hidden="true" /> {{ $t('categories.new') }}
      </BaseButton>
    </header>

    <p v-if="errorMsg" class="err">{{ errorMsg }}</p>
    <p v-if="loading" class="muted">{{ $t('common.loading') }}</p>

    <BaseTable :rows="cats" :columns="cols" :row-key="(r) => r.id" :empty="$t('categories.empty')">
      <template #cell-activo="{ row }">
        <Badge :variant="(row as Categoria).activo ? 'success' : 'neutral'">
          {{ (row as Categoria).activo ? $t('common.enabled') : $t('common.disabled') }}
        </Badge>
      </template>
      <template #cell-acciones="{ row }">
        <div class="row-actions">
          <button class="btn-action btn-edit" @click="openEdit(row as Categoria)">
            <Pencil :size="14" aria-hidden="true" /> {{ $t('common.edit') }}
          </button>
          <button
            class="btn-action"
            :class="(row as Categoria).activo ? 'btn-deactivate' : 'btn-activate'"
            @click="toggleActivo(row as Categoria)"
          >
            <PowerOff v-if="(row as Categoria).activo" :size="14" aria-hidden="true" />
            <Power v-else :size="14" aria-hidden="true" />
            {{ (row as Categoria).activo ? $t('common.deactivate') : $t('common.activate') }}
          </button>
        </div>
      </template>
    </BaseTable>

    <Modal :open="showCreate" :title="$t('categories.new')" @close="showCreate = false">
      <div class="form">
        <BaseInput v-model="cName" :label="$t('common.name')" :placeholder="$t('categories.namePlaceholder')" maxlength="100" required />
        <BaseInput v-model="cDesc" :label="$t('common.description')" :placeholder="$t('categories.descriptionPlaceholder')" maxlength="255" />
        <p v-if="cErr" class="err">{{ cErr }}</p>
      </div>
      <template #footer>
        <BaseButton variant="ghost" @click="showCreate = false">{{ $t('common.cancel') }}</BaseButton>
        <BaseButton :loading="cBusy" @click="submit">{{ $t('common.create') }}</BaseButton>
      </template>
    </Modal>

    <Modal :open="showEdit" :title="$t('categories.edit')" @close="showEdit = false">
      <div class="form">
        <BaseInput v-model="eName" :label="$t('common.name')" :placeholder="$t('categories.namePlaceholder')" maxlength="100" required />
        <BaseInput v-model="eDesc" :label="$t('common.description')" :placeholder="$t('categories.descriptionPlaceholder')" maxlength="255" />
        <label class="check">
          <input type="checkbox" v-model="eActivo" />
          <span>{{ $t('common.enabled') }}</span>
        </label>
        <p v-if="eErr" class="err">{{ eErr }}</p>
      </div>
      <template #footer>
        <BaseButton variant="ghost" @click="showEdit = false">{{ $t('common.cancel') }}</BaseButton>
        <BaseButton :loading="eBusy" @click="submitEdit">{{ $t('common.save') }}</BaseButton>
      </template>
    </Modal>
  </section>
    </main>
    <AppFooter />
  </div>
</template>

<style scoped>
.head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
.head h1 { margin: 0; }
.form { display: flex; flex-direction: column; gap: 0.75rem; }
.link { background: none; border: 0; color: var(--c-primary); cursor: pointer; font: inherit; padding: 0; }
.link:hover { text-decoration: underline; }
.sep { color: var(--c-text-muted); margin: 0 0.4rem; }
.check { display: inline-flex; align-items: center; gap: 0.5rem; font-size: 0.9rem; }

.row-actions {
  display: inline-flex;
  gap: 0.5rem;
  flex-wrap: wrap;
}
.btn-action {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font: inherit;
  font-size: 0.82rem;
  font-weight: 600;
  padding: 0.32rem 0.7rem;
  border-radius: 999px;
  border: 1px solid transparent;
  cursor: pointer;
  line-height: 1;
  transition: background 0.15s, border-color 0.15s, color 0.15s, transform 0.05s;
}
.btn-action:hover { transform: translateY(-1px); }
.btn-action:active { transform: translateY(0); }
.btn-action:focus-visible { outline: 2px solid var(--c-primary); outline-offset: 2px; }

.btn-edit {
  background: rgba(31, 58, 95, 0.08);
  border-color: rgba(31, 58, 95, 0.18);
  color: var(--c-primary);
}
.btn-edit:hover { background: rgba(31, 58, 95, 0.16); }

.btn-deactivate {
  background: rgba(179, 38, 30, 0.08);
  border-color: rgba(179, 38, 30, 0.25);
  color: var(--c-danger);
}
.btn-deactivate:hover { background: rgba(179, 38, 30, 0.16); }

.btn-activate {
  background: rgba(46, 125, 50, 0.10);
  border-color: rgba(46, 125, 50, 0.30);
  color: #2E7D32;
}
.btn-activate:hover { background: rgba(46, 125, 50, 0.18); }
.err { color: var(--c-danger); }
.muted { color: var(--c-text-muted); }
</style>
