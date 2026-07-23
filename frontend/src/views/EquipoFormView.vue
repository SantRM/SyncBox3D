<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import AppFooter from '@/components/AppFooter.vue'
import AppNavbar from '@/components/AppNavbar.vue'
import BaseButton from '@/components/BaseButton.vue'
import BaseInput from '@/components/BaseInput.vue'
import { api } from '@/services/api'
import {
  isAcceptedModelFile,
  MAX_MODEL_BYTES,
  modelName,
  pickMainGltf,
  totalModelBytes,
  uploadRelativePath,
} from '@/services/modelStore'
import type { Categoria, EstadoOperativo, Modelo3D, Nodo } from '@/services/types'

const router = useRouter()
const route = useRoute()
const { t } = useI18n()

const returnRoot = typeof route.query.return_root === 'string' ? route.query.return_root : ''
const returnNode = typeof route.query.return_node === 'string' ? route.query.return_node : ''
const parentNodoId =
  typeof route.query.parent_nodo_id === 'string'
    ? route.query.parent_nodo_id
    : typeof route.query.nodo_id === 'string'
      ? route.query.nodo_id
      : ''

const categorias = ref<Categoria[]>([])
const estados = ref<EstadoOperativo[]>([])
const modelos3d = ref<Modelo3D[]>([])
const parentNode = ref<Nodo | null>(null)

const nombre = ref('')
const fabricante = ref('')
const modelo = ref('')
const serial = ref('')
const categoriaId = ref('')
const estadoId = ref('')
const modelo3dId = ref<string | null>(null)

const uploading = ref(false)
const submitting = ref(false)
const loadingContext = ref(false)
const errorMsg = ref<string | null>(null)

const contextReady = computed(() => !!parentNode.value && parentNode.value.tipo === 'UBICACION')
const contextLabel = computed(() => {
  if (!parentNode.value) return t('equipment.form.noLocationSelected')
  return `${parentNode.value.nombre} / ${parentNode.value.slug}`
})

const valid = computed(
  () => contextReady.value && nombre.value.trim() !== '' && categoriaId.value !== '' && estadoId.value !== '',
)

onMounted(async () => {
  loadingContext.value = true
  errorMsg.value = null
  try {
    const catalogos = Promise.all([api.categorias.list(true), api.estados.list(), api.modelos3d.list()])
    const parent = parentNodoId ? api.nodos.get(parentNodoId) : Promise.resolve(null)
    const [[c, s, m], node] = await Promise.all([catalogos, parent])
    categorias.value = c
    estados.value = s
    modelos3d.value = m
    parentNode.value = node
    if (!parentNodoId) {
      errorMsg.value = t('equipment.form.selectLocationFirst')
    } else if (!node) {
      errorMsg.value = t('equipment.form.resolveLocationError')
    } else if (node.tipo !== 'UBICACION') {
      errorMsg.value = t('equipment.form.locationOnly')
    }
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('equipment.form.loadError')
  } finally {
    loadingContext.value = false
  }
})

function goBack(): void {
  if (returnRoot) {
    void router.push({
      name: 'ubicaciones',
      query: {
        root: returnRoot,
        ...(returnNode ? { node: returnNode } : {}),
      },
    })
    return
  }
  void router.push({ name: 'ubicaciones' })
}

async function uploadModelo(ev: Event): Promise<void> {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (!isAcceptedModelFile(file)) {
    errorMsg.value = t('modelUpload.acceptedOnly')
    input.value = ''
    return
  }
  if (file.size > MAX_MODEL_BYTES) {
    errorMsg.value = t('modelUpload.fileTooLarge', { mb: Math.round(MAX_MODEL_BYTES / 1024 / 1024) })
    input.value = ''
    return
  }
  uploading.value = true
  errorMsg.value = null
  try {
    const m = await api.modelos3d.upload(file, modelName(file))
    modelos3d.value = [m, ...modelos3d.value.filter((x) => x.id !== m.id)]
    modelo3dId.value = m.id
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('modelUpload.uploadError')
  } finally {
    uploading.value = false
    input.value = ''
  }
}

async function uploadModeloFolder(ev: Event): Promise<void> {
  const input = ev.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  if (files.length === 0) return

  const main = pickMainGltf(files)
  if (!main) {
    errorMsg.value = t('modelUpload.noGltfInFolder')
    input.value = ''
    return
  }
  if (totalModelBytes(files) > MAX_MODEL_BYTES) {
    errorMsg.value = t('modelUpload.folderTooLarge', { mb: Math.round(MAX_MODEL_BYTES / 1024 / 1024) })
    input.value = ''
    return
  }

  uploading.value = true
  errorMsg.value = null
  try {
    const assets = files.filter((file) => file !== main)
    const m = await api.modelos3d.upload(main, modelName(main), t('modelUpload.importedFrom', { path: uploadRelativePath(main) }), assets)
    modelos3d.value = [m, ...modelos3d.value.filter((x) => x.id !== m.id)]
    modelo3dId.value = m.id
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('modelUpload.uploadError')
  } finally {
    uploading.value = false
    input.value = ''
  }
}

async function submit(): Promise<void> {
  if (!valid.value || !parentNodoId) {
    errorMsg.value = t('equipment.form.completeRequired')
    return
  }
  submitting.value = true
  errorMsg.value = null
  try {
    const e = await api.equipos.create({
      nombre: nombre.value.trim(),
      categoria_id: categoriaId.value,
      estado_id: estadoId.value,
      fabricante: fabricante.value.trim() || undefined,
      modelo: modelo.value.trim() || undefined,
      serial: serial.value.trim() || undefined,
      parent_nodo_id: parentNodoId,
      modelo_3d_id: modelo3dId.value,
    })
    if (returnRoot) {
      void router.push({
        name: 'ubicaciones',
        query: {
          root: returnRoot,
          node: e.nodo_id ?? returnNode,
        },
      })
      return
    }
    void router.push({ name: 'equipo-detalle', params: { id: e.id } })
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('equipment.form.createError')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="app-shell">
    <AppNavbar />
    <main class="page">
      <section class="equipment-create">
        <header class="head">
          <div>
            <h1>{{ $t('equipment.new') }}</h1>
            <p class="muted">{{ $t('equipment.createUnderLocation') }}</p>
          </div>
          <BaseButton variant="ghost" type="button" @click="goBack">{{ $t('equipment.backToTree') }}</BaseButton>
        </header>

        <p v-if="loadingContext" class="muted">{{ $t('equipment.form.loadingContext') }}</p>

        <div v-if="!loadingContext && !contextReady" class="empty">
          <p v-if="errorMsg" class="err">{{ errorMsg }}</p>
          <BaseButton type="button" @click="goBack">{{ $t('dashboard.manageLocations') }}</BaseButton>
        </div>

        <form v-else class="form" @submit.prevent="submit">
          <section class="context">
            <span>{{ $t('equipment.parentLocation') }}</span>
            <strong>{{ contextLabel }}</strong>
            <code v-if="parentNode">{{ parentNode.path }}</code>
          </section>

          <BaseInput v-model="nombre" :label="$t('common.name')" :placeholder="$t('equipment.form.namePlaceholder')" maxlength="150" required />
          <div class="grid">
            <label class="select">
              <span>{{ $t('common.category') }} *</span>
              <select v-model="categoriaId" required>
                <option value="" disabled>{{ $t('equipment.form.selectCategory') }}</option>
                <option v-for="c in categorias" :key="c.id" :value="c.id">{{ c.nombre }}</option>
              </select>
            </label>
            <label class="select">
              <span>{{ $t('equipment.initialStatus') }} *</span>
              <select v-model="estadoId" required>
                <option value="" disabled>{{ $t('equipment.form.selectInitialStatus') }}</option>
                <option v-for="e in estados" :key="e.id" :value="e.id">{{ e.nombre }}</option>
              </select>
            </label>
          </div>
          <div class="grid">
            <BaseInput v-model="fabricante" :label="$t('common.manufacturer')" :placeholder="$t('equipment.form.manufacturerPlaceholder')" maxlength="120" />
            <BaseInput v-model="modelo" :label="$t('common.model')" :placeholder="$t('equipment.form.modelPlaceholder')" maxlength="120" />
          </div>
          <BaseInput v-model="serial" :label="$t('common.serial')" :placeholder="$t('equipment.form.serialPlaceholder')" maxlength="120" />

          <fieldset class="box">
            <legend>{{ $t('equipment.model3d') }}</legend>
            <label class="select">
              <span>{{ $t('equipment.existingModel') }}</span>
              <select v-model="modelo3dId">
                <option :value="null">{{ $t('equipment.noModel') }}</option>
                <option v-for="m in modelos3d" :key="m.id" :value="m.id">{{ m.nombre }}</option>
              </select>
            </label>
            <label class="upload">
              <span>{{ $t('modelUpload.uploadGlb') }}</span>
              <input type="file" accept=".glb,.gltf,model/gltf-binary,model/gltf+json" :disabled="uploading" @change="uploadModelo" />
              <small class="muted">{{ $t('modelUpload.folderHint') }}</small>
            </label>
            <label class="upload">
              <span>{{ $t('modelUpload.uploadFolder') }}</span>
              <input type="file" webkitdirectory directory multiple :disabled="uploading" @change="uploadModeloFolder" />
              <small v-if="uploading" class="muted">{{ $t('modelUpload.processing') }}</small>
            </label>
          </fieldset>

          <p v-if="errorMsg" class="err">{{ errorMsg }}</p>

          <div class="actions">
            <BaseButton variant="ghost" type="button" @click="goBack">{{ $t('common.cancel') }}</BaseButton>
            <BaseButton type="submit" :loading="submitting" :disabled="!valid">{{ $t('equipment.create') }}</BaseButton>
          </div>
        </form>
      </section>
    </main>
    <AppFooter />
  </div>
</template>

<style scoped>
.equipment-create { max-width: 860px; }
.head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 1rem;
}
.head h1 { margin: 0; }
.form { display: flex; flex-direction: column; gap: 0.85rem; }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(230px, 1fr)); gap: 0.85rem; }
.context {
  display: grid;
  gap: 0.25rem;
  padding: 0.75rem 0.9rem;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  background: var(--c-surface-2);
}
.context span,
.muted { color: var(--c-text-muted); margin: 0; font-size: 0.85rem; }
.context code {
  color: var(--c-text-muted);
  font-size: 0.78rem;
  overflow-wrap: anywhere;
}
.select { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.875rem; font-weight: 600; }
.select select {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 0.55rem 0.5rem;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  background: var(--c-surface);
  color: var(--c-text);
  font: inherit;
}
.box {
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  padding: 0.85rem 1rem;
  display: grid;
  gap: 0.65rem;
}
.box legend { font-weight: 600; padding: 0 0.4rem; }
.upload { display: grid; gap: 0.35rem; font-size: 0.875rem; font-weight: 600; }
.upload input { max-width: 100%; }
.err { color: var(--c-danger); }
.actions { display: flex; gap: 0.5rem; justify-content: flex-end; margin-top: 0.5rem; }
.empty {
  display: grid;
  gap: 0.75rem;
  justify-items: start;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  padding: 1rem;
}
@media (max-width: 600px) {
  .head { flex-direction: column; }
  .grid { grid-template-columns: 1fr; }
}
</style>
