<script setup lang="ts">
import { Pencil, Plus, PowerOff } from '@lucide/vue'
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import AppFooter from '@/components/AppFooter.vue'
import AppNavbar from '@/components/AppNavbar.vue'
import Badge from '@/components/Badge.vue'
import BaseButton from '@/components/BaseButton.vue'
import BaseInput from '@/components/BaseInput.vue'
import BaseTable from '@/components/BaseTable.vue'
import Modal from '@/components/Modal.vue'
import { api } from '@/services/api'
import type { PublicUsuario, Role } from '@/services/types'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const { t } = useI18n()

const users = ref<PublicUsuario[]>([])
const loading = ref(false)
const errorMsg = ref<string | null>(null)

const showCreate = ref(false)
const fNombre = ref('')
const fCorreo = ref('')
const fPassword = ref('')
const fRol = ref<Role>('OPERADOR')
const formError = ref<string | null>(null)
const formBusy = ref(false)

const showEdit = ref(false)
const editTarget = ref<PublicUsuario | null>(null)
const eNombre = ref('')
const eRol = ref<Role>('OPERADOR')
const eActivo = ref(true)
const editError = ref<string | null>(null)
const editBusy = ref(false)

async function load(): Promise<void> {
  loading.value = true
  errorMsg.value = null
  try {
    users.value = await api.usuarios.list()
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('users.loadError')
  } finally {
    loading.value = false
  }
}

function openEdit(u: PublicUsuario): void {
  editTarget.value = u
  eNombre.value = u.nombre
  eRol.value = u.rol
  eActivo.value = u.activo
  editError.value = null
  showEdit.value = true
}

async function submitCreate(): Promise<void> {
  if (!fNombre.value.trim() || !fCorreo.value.trim() || fPassword.value.length < 10) {
    formError.value = t('users.validationRequired')
    return
  }
  formBusy.value = true
  formError.value = null
  try {
    await api.usuarios.create({
      nombre: fNombre.value.trim(),
      correo: fCorreo.value.trim().toLowerCase(),
      password: fPassword.value,
      rol: fRol.value,
    })
    showCreate.value = false
    fNombre.value = ''; fCorreo.value = ''; fPassword.value = ''; fRol.value = 'OPERADOR'
    await load()
  } catch (e) {
    formError.value = (e as { message?: string }).message ?? t('users.createError')
  } finally {
    formBusy.value = false
  }
}

async function submitEdit(): Promise<void> {
  if (!editTarget.value) return
  editBusy.value = true
  editError.value = null
  try {
    const target = editTarget.value
    const patch: { nombre?: string; rol?: Role; activo?: boolean } = {}
    const trimmed = eNombre.value.trim()
    if (trimmed && trimmed !== target.nombre) patch.nombre = trimmed
    if (eRol.value !== target.rol) patch.rol = eRol.value
    if (eActivo.value !== target.activo) patch.activo = eActivo.value
    if (Object.keys(patch).length > 0) {
      await api.usuarios.update(target.id, patch)
    }
    showEdit.value = false
    await load()
  } catch (e) {
    editError.value = (e as { message?: string }).message ?? t('users.updateError')
  } finally {
    editBusy.value = false
  }
}

async function deactivate(u: PublicUsuario): Promise<void> {
  if (u.id === auth.user?.id) return
  if (!confirm(t('users.deactivateConfirmName', { name: u.nombre }))) return
  try {
    await api.usuarios.deactivate(u.id)
    await load()
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('users.deactivateError')
  }
}

onMounted(load)

const cols = computed(() => [
  { key: 'nombre', label: t('common.name') },
  { key: 'correo', label: t('auth.login.email') },
  { key: 'rol', label: t('dashboard.role') },
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
      <h1>{{ $t('users.title') }}</h1>
      <BaseButton @click="showCreate = true">
        <Plus :size="16" aria-hidden="true" /> {{ $t('users.new') }}
      </BaseButton>
    </header>

    <p v-if="errorMsg" class="err">{{ errorMsg }}</p>
    <p v-if="loading" class="muted">{{ $t('common.loading') }}</p>

    <BaseTable :rows="users" :columns="cols" :row-key="(r) => r.id" :empty="$t('users.empty')">
      <template #cell-rol="{ row }">
        <Badge :variant="(row as PublicUsuario).rol === 'ADMINISTRADOR' ? 'info' : 'neutral'">{{ $t(`roles.${(row as PublicUsuario).rol}`) }}</Badge>
      </template>
      <template #cell-activo="{ row }">
        <Badge :variant="(row as PublicUsuario).activo ? 'success' : 'danger'">
          {{ (row as PublicUsuario).activo ? $t('common.active') : $t('common.inactive') }}
        </Badge>
      </template>
      <template #cell-acciones="{ row }">
        <button class="link" @click="openEdit(row as PublicUsuario)">
          <Pencil :size="14" aria-hidden="true" /> {{ $t('common.edit') }}
        </button>
        <button
          class="link link--danger"
          :disabled="(row as PublicUsuario).id === auth.user?.id"
          @click="deactivate(row as PublicUsuario)"
        >
          <PowerOff :size="14" aria-hidden="true" /> {{ $t('common.deactivate') }}
        </button>
      </template>
    </BaseTable>

    <Modal :open="showCreate" :title="$t('users.new')" @close="showCreate = false">
      <div class="form">
        <BaseInput v-model="fNombre" :label="$t('common.name')" :placeholder="$t('users.namePlaceholder')" maxlength="120" required />
        <BaseInput v-model="fCorreo" :label="$t('auth.login.email')" type="email" :placeholder="$t('users.emailPlaceholder')" maxlength="180" required autocomplete="off" />
        <BaseInput v-model="fPassword" :label="$t('auth.password.next')" type="password" :placeholder="$t('users.passwordPlaceholder')" minlength="10" required autocomplete="new-password" />
        <label class="select">
          <span>{{ $t('dashboard.role') }}</span>
          <select v-model="fRol">
            <option value="ADMINISTRADOR">{{ $t('roles.ADMINISTRADOR') }}</option>
            <option value="OPERADOR">{{ $t('roles.OPERADOR') }}</option>
            <option value="CONSULTA">{{ $t('roles.CONSULTA') }}</option>
          </select>
        </label>
        <p v-if="formError" class="err">{{ formError }}</p>
      </div>
      <template #footer>
        <BaseButton variant="ghost" @click="showCreate = false">{{ $t('common.cancel') }}</BaseButton>
        <BaseButton :loading="formBusy" @click="submitCreate">{{ $t('common.create') }}</BaseButton>
      </template>
    </Modal>

    <Modal :open="showEdit" :title="$t('users.edit')" @close="showEdit = false">
      <div v-if="editTarget" class="form">
        <BaseInput v-model="eNombre" :label="$t('common.name')" :placeholder="$t('users.namePlaceholder')" maxlength="120" required />
        <label class="select">
          <span>{{ $t('dashboard.role') }}</span>
          <select v-model="eRol" :disabled="editTarget.id === auth.user?.id">
            <option value="ADMINISTRADOR">{{ $t('roles.ADMINISTRADOR') }}</option>
            <option value="OPERADOR">{{ $t('roles.OPERADOR') }}</option>
            <option value="CONSULTA">{{ $t('roles.CONSULTA') }}</option>
          </select>
        </label>
        <label class="check">
          <input v-model="eActivo" type="checkbox" :disabled="editTarget.id === auth.user?.id" />
          <span>{{ $t('users.activeAccount') }}</span>
        </label>
        <p v-if="editTarget.id === auth.user?.id" class="muted">{{ $t('users.selfEditHint') }}</p>
        <p v-if="editError" class="err">{{ editError }}</p>
      </div>
      <template #footer>
        <BaseButton variant="ghost" @click="showEdit = false">{{ $t('common.cancel') }}</BaseButton>
        <BaseButton :loading="editBusy" @click="submitEdit">{{ $t('common.save') }}</BaseButton>
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
.select { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.875rem; font-weight: 600; }
.select select { width: 100%; min-width: 0; box-sizing: border-box; padding: 0.55rem 0.5rem; border: 1px solid var(--c-border); border-radius: var(--radius-md); background: var(--c-surface); color: var(--c-text); font: inherit; }
.check { display: flex; align-items: center; gap: 0.5rem; }
.link { background: none; border: 0; padding: 0; color: var(--c-primary); cursor: pointer; font: inherit; margin-right: 0.75rem; }
.link:hover { text-decoration: underline; }
.link--danger { color: var(--c-danger); }
.link[disabled] { opacity: 0.4; cursor: not-allowed; }
.err { color: var(--c-danger); }
.muted { color: var(--c-text-muted); font-size: 0.875rem; }
</style>
