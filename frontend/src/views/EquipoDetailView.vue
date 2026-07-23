<script setup lang="ts">
import { ArrowLeft, ArrowRight } from '@lucide/vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'

import AppFooter from '@/components/AppFooter.vue'
import AppNavbar from '@/components/AppNavbar.vue'
import Badge from '@/components/Badge.vue'
import BaseButton from '@/components/BaseButton.vue'
import BaseInput from '@/components/BaseInput.vue'
import Modal from '@/components/Modal.vue'
import RoleGate from '@/components/RoleGate.vue'
import Viewer3D from '@/components/Viewer3D.vue'
import { api } from '@/services/api'
import { dateLocale } from '@/i18n'
import {
  isAcceptedModelFile,
  MAX_MODEL_BYTES,
  modelName,
  pickMainGltf,
  totalModelBytes,
  uploadRelativePath,
} from '@/services/modelStore'
import type {
  CambioEntry,
  Categoria,
  Equipo,
  EstadoHistorialEntry,
  EstadoOperativo,
  FichaTecnica,
  Modelo3D,
} from '@/services/types'

const props = defineProps<{ id: string }>()
const router = useRouter()
const { locale, t } = useI18n()

const equipo = ref<Equipo | null>(null)
const categoria = ref<Categoria | null>(null)
const estados = ref<EstadoOperativo[]>([])
const loadError = ref<string | null>(null)
const loading = ref(true)

// Modelo 3D persistido en backend; el equipo solo apunta a modelo_3d_id.
const modeloUrl = ref<string | null>(null)

async function loadModelo(): Promise<void> {
  if (modeloUrl.value?.startsWith('blob:')) {
    URL.revokeObjectURL(modeloUrl.value)
  }
  modeloUrl.value = null
  const mid = equipo.value?.modelo_3d_id
  if (mid) {
    modeloUrl.value = api.modelos3d.fileUrl(mid)
  }
}

// Reemplazo de modelo desde el detalle.
const showModelo = ref(false)
type ModeloOpcion = 'existente' | 'archivo' | 'ninguno'
const modeloOpcion = ref<ModeloOpcion>('existente')
const modelos3d = ref<Modelo3D[]>([])
const modelo3dId = ref('')
const archivo = ref<File | null>(null)
const archivoAssets = ref<File[]>([])
const modeloError = ref<string | null>(null)
const modeloBusy = ref(false)
const modelosLoading = ref(false)

async function loadModelos3d(): Promise<void> {
  modelosLoading.value = true
  try {
    modelos3d.value = await api.modelos3d.list()
  } finally {
    modelosLoading.value = false
  }
}

async function openModeloModal(): Promise<void> {
  modeloError.value = null
  archivo.value = null
  archivoAssets.value = []
  modeloOpcion.value = equipo.value?.modelo_3d_id ? 'existente' : 'archivo'
  modelo3dId.value = equipo.value?.modelo_3d_id ?? ''
  showModelo.value = true
  try {
    await loadModelos3d()
  } catch (e) {
    modeloError.value = (e as { message?: string }).message ?? t('modelUpload.loadExistingError')
  }
}

function onFile(ev: Event): void {
  modeloError.value = null
  archivoAssets.value = []
  const f = (ev.target as HTMLInputElement).files?.[0] ?? null
  if (!f) { archivo.value = null; return }
  if (!isAcceptedModelFile(f)) {
    modeloError.value = t('modelUpload.acceptedOnly')
    archivo.value = null; return
  }
  if (f.size > MAX_MODEL_BYTES) {
    modeloError.value = t('modelUpload.fileTooLarge', { mb: Math.round(MAX_MODEL_BYTES / 1024 / 1024) })
    archivo.value = null; return
  }
  archivo.value = f
}

function onFolder(ev: Event): void {
  modeloError.value = null
  const input = ev.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  if (files.length === 0) {
    archivo.value = null
    archivoAssets.value = []
    return
  }
  const main = pickMainGltf(files)
  if (!main) {
    modeloError.value = t('modelUpload.noGltfInFolder')
    archivo.value = null
    archivoAssets.value = []
    input.value = ''
    return
  }
  if (totalModelBytes(files) > MAX_MODEL_BYTES) {
    modeloError.value = t('modelUpload.folderTooLarge', { mb: Math.round(MAX_MODEL_BYTES / 1024 / 1024) })
    archivo.value = null
    archivoAssets.value = []
    input.value = ''
    return
  }
  archivo.value = main
  archivoAssets.value = files.filter((file) => file !== main)
}

async function applyModelo(): Promise<void> {
  modeloBusy.value = true
  modeloError.value = null
  try {
    if (modeloOpcion.value === 'archivo') {
      if (!archivo.value) { modeloError.value = t('modelUpload.selectFile'); return }
      const uploaded = await api.modelos3d.upload(
        archivo.value,
        modelName(archivo.value),
        archivoAssets.value.length > 0 ? t('modelUpload.importedFrom', { path: uploadRelativePath(archivo.value) }) : undefined,
        archivoAssets.value,
      )
      modelos3d.value = [uploaded, ...modelos3d.value.filter((m) => m.id !== uploaded.id)]
      equipo.value = await api.equipos.setModelo3D(props.id, uploaded.id)
    } else if (modeloOpcion.value === 'existente') {
      if (!modelo3dId.value) { modeloError.value = t('modelUpload.selectExistingRequired'); return }
      equipo.value = await api.equipos.setModelo3D(props.id, modelo3dId.value)
    } else {
      equipo.value = await api.equipos.setModelo3D(props.id, null)
    }
    showModelo.value = false
    archivo.value = null
    archivoAssets.value = []
    await loadModelo()
  } catch (e) {
    modeloError.value = (e as { message?: string }).message ?? t('modelUpload.saveError')
  } finally {
    modeloBusy.value = false
  }
}

const tab = ref<'identificacion' | 'specs' | 'historial'>('identificacion')

// --- Edición de identificación ---
const categoriasList = ref<Categoria[]>([])
const idEditing = ref(false)
const idSaving = ref(false)
const idError = ref<string | null>(null)
const idForm = ref<{
  nombre: string
  fabricante: string
  modelo: string
  serial: string
  ubicacion: string
  categoria_id: string
}>({ nombre: '', fabricante: '', modelo: '', serial: '', ubicacion: '', categoria_id: '' })

function openEditId(): void {
  if (!equipo.value) return
  idForm.value = {
    nombre: equipo.value.nombre,
    fabricante: equipo.value.fabricante ?? '',
    modelo: equipo.value.modelo ?? '',
    serial: equipo.value.serial ?? '',
    ubicacion: equipo.value.ubicacion ?? '',
    categoria_id: equipo.value.categoria_id,
  }
  idError.value = null
  idEditing.value = true
}

async function saveId(): Promise<void> {
  if (!equipo.value) return
  const before = equipo.value
  const trimmedNombre = idForm.value.nombre.trim()
  if (!trimmedNombre) {
    idError.value = t('validation.required')
    return
  }
  const patch: Record<string, unknown> = {}
  if (trimmedNombre !== before.nombre) patch.nombre = trimmedNombre
  if (idForm.value.fabricante !== (before.fabricante ?? '')) patch.fabricante = idForm.value.fabricante
  if (idForm.value.modelo !== (before.modelo ?? '')) patch.modelo = idForm.value.modelo
  if (idForm.value.serial !== (before.serial ?? '')) patch.serial = idForm.value.serial
  if (idForm.value.ubicacion !== (before.ubicacion ?? '')) patch.ubicacion = idForm.value.ubicacion
  if (idForm.value.categoria_id !== before.categoria_id) patch.categoria_id = idForm.value.categoria_id

  if (Object.keys(patch).length === 0) {
    idEditing.value = false
    return
  }

  idSaving.value = true
  idError.value = null
  try {
    const updated = await api.equipos.update(props.id, patch)
    equipo.value = updated
    // Refrescar categoría asociada y nombre visible.
    categoria.value = categoriasList.value.find((c) => c.id === updated.categoria_id) ?? categoria.value
    idEditing.value = false
    void loadHistorial()
  } catch (e) {
    idError.value = (e as { message?: string }).message ?? t('equipment.saveError')
  } finally {
    idSaving.value = false
  }
}

// --- Especificaciones (ficha técnica) ---
const ficha = ref<FichaTecnica | null>(null)
const fichaLoading = ref(false)
const fichaSaving = ref(false)
const fichaError = ref<string | null>(null)
const fichaForm = ref<{
  peso: string
  potencia: string
  dimensiones: string
  anio: string
  observaciones: string
}>({ peso: '', potencia: '', dimensiones: '', anio: '', observaciones: '' })
const fichaEditing = ref(false)

async function loadFicha(): Promise<void> {
  fichaLoading.value = true
  fichaError.value = null
  try {
    ficha.value = await api.equipos.getFicha(props.id)
    fichaForm.value = {
      peso: ficha.value?.peso != null ? String(ficha.value.peso) : '',
      potencia: ficha.value?.potencia != null ? String(ficha.value.potencia) : '',
      dimensiones: ficha.value?.dimensiones ?? '',
      anio: ficha.value?.anio != null ? String(ficha.value.anio) : '',
      observaciones: ficha.value?.observaciones ?? '',
    }
  } catch (e) {
    fichaError.value = (e as { message?: string }).message ?? t('equipment.detail.sheetLoadError')
  } finally {
    fichaLoading.value = false
  }
}

async function saveFicha(): Promise<void> {
  fichaSaving.value = true
  fichaError.value = null
  try {
    const payload: Partial<FichaTecnica> = {
      peso: fichaForm.value.peso ? Number(fichaForm.value.peso) : null,
      potencia: fichaForm.value.potencia ? Number(fichaForm.value.potencia) : null,
      dimensiones: fichaForm.value.dimensiones || '',
      anio: fichaForm.value.anio ? Number(fichaForm.value.anio) : null,
      observaciones: fichaForm.value.observaciones || '',
      atributos_extra: ficha.value?.atributos_extra ?? {},
    }
    ficha.value = await api.equipos.upsertFicha(props.id, payload)
    fichaEditing.value = false
    // Refrescar historial para reflejar los cambios.
    void loadHistorial()
  } catch (e) {
    fichaError.value = (e as { message?: string }).message ?? t('equipment.detail.sheetSaveError')
  } finally {
    fichaSaving.value = false
  }
}

// --- Historial ---
const histEstados = ref<EstadoHistorialEntry[]>([])
const histCambios = ref<CambioEntry[]>([])
const histLoading = ref(false)
const histError = ref<string | null>(null)

async function loadHistorial(): Promise<void> {
  histLoading.value = true
  histError.value = null
  try {
    const r = await api.equipos.historial(props.id)
    histEstados.value = r.estados ?? []
    histCambios.value = r.cambios ?? []
  } catch (e) {
    histError.value = (e as { message?: string }).message ?? t('equipment.detail.historyLoadError')
  } finally {
    histLoading.value = false
  }
}

function fmtDate(s: string): string {
  return new Date(s).toLocaleString(dateLocale(locale.value))
}

function fieldLabel(c: string): string {
  const map: Record<string, string> = {
    nombre: t('common.name'),
    fabricante: t('common.manufacturer'),
    modelo: t('common.model'),
    serial: t('common.serial'),
    ubicacion: t('locations.title'),
    categoria_id: t('common.category'),
    peso: t('equipment.detail.weightKg'),
    potencia: t('equipment.detail.powerKw'),
    dimensiones: t('equipment.detail.dimensions'),
    anio: t('equipment.detail.year'),
    observaciones: t('equipment.detail.notes'),
  }
  return map[c] ?? c
}

// Cambio de estado
const showState = ref(false)
const newEstadoId = ref('')
const motivo = ref('')
const stateError = ref<string | null>(null)
const stateBusy = ref(false)

const estadoActual = computed(() =>
  equipo.value ? estados.value.find((e) => e.id === equipo.value!.estado_id) ?? null : null,
)

async function load(): Promise<void> {
  loading.value = true
  loadError.value = null
  try {
    const [e, ests] = await Promise.all([api.equipos.get(props.id), api.estados.list()])
    equipo.value = e
    estados.value = ests
    try {
      const cats = await api.categorias.list(false)
      categoriasList.value = cats
      categoria.value = cats.find((c) => c.id === e.categoria_id) ?? null
    } catch {
      categoria.value = null
    }
    await loadModelo()
    await Promise.all([loadFicha(), loadHistorial()])
  } catch (e) {
    loadError.value = (e as { message?: string }).message ?? t('equipment.loadError')
  } finally {
    loading.value = false
  }
}

async function applyState(): Promise<void> {
  if (!equipo.value || !newEstadoId.value || !motivo.value.trim()) {
    stateError.value = t('equipment.detail.stateRequired')
    return
  }
  stateBusy.value = true
  stateError.value = null
  try {
    await api.equipos.changeState(equipo.value.id, newEstadoId.value, motivo.value.trim())
    showState.value = false
    motivo.value = ''
    newEstadoId.value = ''
    await load()
  } catch (e) {
    stateError.value = (e as { message?: string }).message ?? t('equipment.detail.stateError')
  } finally {
    stateBusy.value = false
  }
}

async function softDelete(): Promise<void> {
  if (!equipo.value) return
  if (!confirm(t('equipment.deleteConfirm'))) return
  try {
    await api.equipos.delete(equipo.value.id)
    void router.push({ name: 'equipos' })
  } catch (e) {
    loadError.value = (e as { message?: string }).message ?? t('equipment.detail.deleteError')
  }
}

onMounted(load)
onBeforeUnmount(() => {
  if (modeloUrl.value?.startsWith('blob:')) URL.revokeObjectURL(modeloUrl.value)
})
</script>

<template>
  <div class="app-shell">
    <AppNavbar />
    <main class="page">
  <section v-if="loading">{{ $t('common.loading') }}</section>
  <section v-else-if="loadError" class="err">{{ loadError }}</section>
  <section v-else-if="equipo">
    <header class="head">
      <div>
        <h1>{{ equipo.nombre }}</h1>
        <p class="head__sub">
          <Badge :color="estadoActual?.color">{{ estadoActual?.nombre ?? $t('alerts.popup.noStatus') }}</Badge>
          <span v-if="categoria">· {{ categoria.nombre }}</span>
        </p>
      </div>
      <RoleGate :roles="['ADMINISTRADOR', 'OPERADOR']">
        <div class="actions">
          <BaseButton variant="secondary" @click="openModeloModal">{{ $t('equipment.model3d') }}</BaseButton>
          <BaseButton variant="secondary" @click="showState = true">{{ $t('equipment.changeStatus') }}</BaseButton>
          <BaseButton variant="danger" @click="softDelete">{{ $t('equipment.retire') }}</BaseButton>
        </div>
      </RoleGate>
    </header>

    <Viewer3D :src="modeloUrl ?? undefined" />

    <nav class="tabs">
      <button :class="{ active: tab === 'identificacion' }" @click="tab = 'identificacion'">{{ $t('equipment.identification') }}</button>
      <button :class="{ active: tab === 'specs' }" @click="tab = 'specs'">{{ $t('equipment.specifications') }}</button>
      <button :class="{ active: tab === 'historial' }" @click="tab = 'historial'">{{ $t('equipment.history') }}</button>
    </nav>

    <div v-if="tab === 'identificacion'" class="panel">
      <div class="panel-head">
        <h3>{{ $t('equipment.identification') }}</h3>
        <RoleGate :roles="['ADMINISTRADOR', 'OPERADOR']">
          <BaseButton v-if="!idEditing" variant="secondary" @click="openEditId">{{ $t('common.edit') }}</BaseButton>
        </RoleGate>
      </div>

      <template v-if="!idEditing">
        <dl class="def">
          <dt>{{ $t('common.manufacturer') }}</dt><dd>{{ equipo.fabricante || '—' }}</dd>
          <dt>{{ $t('common.model') }}</dt><dd>{{ equipo.modelo || '—' }}</dd>
          <dt>{{ $t('common.serial') }}</dt><dd>{{ equipo.serial || '—' }}</dd>
          <dt>{{ $t('locations.title') }}</dt><dd>{{ equipo.ubicacion || '—' }}</dd>
          <dt>{{ $t('common.category') }}</dt><dd>{{ categoria?.nombre || '—' }}</dd>
          <dt>{{ $t('equipment.detail.statusSince') }}</dt><dd>{{ fmtDate(equipo.estado_desde) }}</dd>
          <dt>{{ $t('common.createdAt') }}</dt><dd>{{ fmtDate(equipo.created_at) }}</dd>
          <dt>{{ $t('common.updatedAt') }}</dt><dd>{{ fmtDate(equipo.updated_at) }}</dd>
        </dl>
      </template>

      <template v-else>
        <div class="form-grid">
          <BaseInput v-model="idForm.nombre" :label="$t('common.name')" :placeholder="$t('equipment.form.namePlaceholder')" maxlength="150" required />
          <BaseInput v-model="idForm.fabricante" :label="$t('common.manufacturer')" :placeholder="$t('equipment.form.manufacturerPlaceholder')" maxlength="120" />
          <BaseInput v-model="idForm.modelo" :label="$t('common.model')" :placeholder="$t('equipment.form.modelPlaceholder')" maxlength="120" />
          <BaseInput v-model="idForm.serial" :label="$t('common.serial')" :placeholder="$t('equipment.form.serialPlaceholder')" maxlength="120" />
          <BaseInput v-model="idForm.ubicacion" :label="$t('locations.title')" :placeholder="$t('equipment.detail.locationPlaceholder')" maxlength="180" />
          <label class="select">
            <span>{{ $t('common.category') }}</span>
            <select v-model="idForm.categoria_id" required>
              <option v-for="c in categoriasList" :key="c.id" :value="c.id">{{ c.nombre }}</option>
            </select>
          </label>
        </div>
        <p v-if="idError" class="err">{{ idError }}</p>
        <div class="form-actions">
          <BaseButton variant="ghost" :disabled="idSaving" @click="idEditing = false">{{ $t('common.cancel') }}</BaseButton>
          <BaseButton :loading="idSaving" @click="saveId">{{ $t('common.save') }}</BaseButton>
        </div>
      </template>
    </div>

    <div v-else-if="tab === 'specs'" class="panel">
      <div class="panel-head">
        <h3>{{ $t('equipment.detail.technicalSpecs') }}</h3>
        <RoleGate :roles="['ADMINISTRADOR', 'OPERADOR']">
          <BaseButton v-if="!fichaEditing" variant="secondary" @click="fichaEditing = true">
            {{ ficha ? $t('common.edit') : $t('equipment.detail.addSheet') }}
          </BaseButton>
        </RoleGate>
      </div>
      <p v-if="fichaLoading" class="muted">{{ $t('common.loading') }}</p>
      <p v-if="fichaError" class="err">{{ fichaError }}</p>

      <template v-if="!fichaEditing">
        <p v-if="!ficha && !fichaLoading" class="muted">
          {{ $t('equipment.detail.noSheet') }}
        </p>
        <dl v-else-if="ficha" class="def">
          <dt>{{ $t('equipment.detail.weight') }}</dt><dd>{{ ficha.peso != null ? `${ficha.peso} kg` : '—' }}</dd>
          <dt>{{ $t('equipment.detail.power') }}</dt><dd>{{ ficha.potencia != null ? `${ficha.potencia} kW` : '—' }}</dd>
          <dt>{{ $t('equipment.detail.dimensions') }}</dt><dd>{{ ficha.dimensiones || '—' }}</dd>
          <dt>{{ $t('equipment.detail.year') }}</dt><dd>{{ ficha.anio ?? '—' }}</dd>
          <dt>{{ $t('equipment.detail.notes') }}</dt><dd class="pre">{{ ficha.observaciones || '—' }}</dd>
        </dl>
      </template>

      <form v-else class="form" @submit.prevent="saveFicha">
        <div class="grid">
          <BaseInput v-model="fichaForm.peso" type="number" :label="$t('equipment.detail.weightKg')" :placeholder="$t('equipment.detail.weightPlaceholder')" min="0" step="0.01" inputmode="decimal" />
          <BaseInput v-model="fichaForm.potencia" type="number" :label="$t('equipment.detail.powerKw')" :placeholder="$t('equipment.detail.powerPlaceholder')" min="0" step="0.01" inputmode="decimal" />
        </div>
        <div class="grid">
          <BaseInput v-model="fichaForm.dimensiones" :label="$t('equipment.detail.dimensions')" :placeholder="$t('equipment.detail.dimensionsPlaceholder')" maxlength="80" />
          <BaseInput v-model="fichaForm.anio" type="number" :label="$t('equipment.detail.year')" :placeholder="$t('equipment.detail.yearPlaceholder')" min="1900" max="2100" step="1" inputmode="numeric" />
        </div>
        <label class="select">
          <span>{{ $t('equipment.detail.notes') }}</span>
          <textarea v-model="fichaForm.observaciones" rows="4" :placeholder="$t('equipment.detail.notesPlaceholder')" />
        </label>
        <div class="form-actions">
          <BaseButton type="button" variant="ghost" @click="fichaEditing = false; void loadFicha()">{{ $t('common.cancel') }}</BaseButton>
          <BaseButton type="submit" :loading="fichaSaving">{{ $t('common.save') }}</BaseButton>
        </div>
      </form>
    </div>

    <div v-else class="panel">
      <h3>{{ $t('equipment.detail.stateHistory') }}</h3>
      <p v-if="histLoading" class="muted">{{ $t('common.loading') }}</p>
      <p v-if="histError" class="err">{{ histError }}</p>
      <p v-else-if="histEstados.length === 0" class="muted">{{ $t('equipment.detail.noMovements') }}</p>
      <ol v-else class="timeline">
        <li v-for="h in histEstados" :key="h.id">
          <div class="timeline-head">
            <Badge :color="h.estado_nuevo_color">{{ h.estado_nuevo }}</Badge>
            <span v-if="h.estado_anterior" class="muted small inline-icon">
              <ArrowLeft :size="14" aria-hidden="true" /> {{ h.estado_anterior }}
            </span>
            <span v-else class="muted small">({{ $t('equipment.detail.createdEvent') }})</span>
            <span class="muted small spacer">{{ fmtDate(h.fecha) }}</span>
          </div>
          <p class="small">
            {{ $t('equipment.detail.by') }} <strong>{{ h.usuario_nombre }}</strong>
            <template v-if="h.motivo"> · <em>{{ h.motivo }}</em></template>
          </p>
        </li>
      </ol>

      <h3 class="mt">{{ $t('equipment.detail.fieldChanges') }}</h3>
      <p v-if="histCambios.length === 0" class="muted">{{ $t('equipment.detail.noFieldChanges') }}</p>
      <ul v-else class="changes">
        <li v-for="c in histCambios" :key="c.id">
          <span class="muted small">{{ fmtDate(c.fecha) }}</span>
          <strong>{{ fieldLabel(c.campo) }}</strong>:
          <span class="muted">{{ c.valor_anterior || '—' }}</span>
          <ArrowRight :size="14" aria-hidden="true" class="inline-arrow" />
          <span>{{ c.valor_nuevo || '—' }}</span>
          <span class="muted small">{{ $t('equipment.detail.by') }} {{ c.usuario_nombre }}</span>
        </li>
      </ul>
    </div>

    <Modal :open="showState" :title="$t('equipment.changeStatus')" @close="showState = false">
      <div class="form">
        <label class="select">
          <span>{{ $t('equipment.detail.newStatus') }} *</span>
          <select v-model="newEstadoId">
            <option value="" disabled>{{ $t('common.select') }}</option>
            <option v-for="e in estados" :key="e.id" :value="e.id" :disabled="e.id === equipo.estado_id">{{ e.nombre }}</option>
          </select>
        </label>
        <label class="select">
          <span>{{ $t('equipment.detail.reason') }} *</span>
          <textarea v-model="motivo" rows="3" maxlength="255" :placeholder="$t('equipment.detail.reasonPlaceholder')" />
        </label>
        <p v-if="stateError" class="err">{{ stateError }}</p>
      </div>
      <template #footer>
        <BaseButton variant="ghost" @click="showState = false">{{ $t('common.cancel') }}</BaseButton>
        <BaseButton :loading="stateBusy" @click="applyState">{{ $t('common.confirm') }}</BaseButton>
      </template>
    </Modal>

    <Modal :open="showModelo" :title="$t('equipment.detail.modelModalTitle')" @close="showModelo = false">
      <div class="form">
        <label class="radio">
          <input v-model="modeloOpcion" type="radio" value="existente" />
          <span>{{ $t('modelUpload.useExisting') }}</span>
        </label>
        <div v-if="modeloOpcion === 'existente'" class="sub">
          <label class="select">
            <span>{{ $t('common.model') }}</span>
            <select v-model="modelo3dId" :disabled="modelosLoading">
              <option value="" disabled>{{ modelosLoading ? $t('common.loading') : $t('modelUpload.selectExisting') }}</option>
              <option v-for="m in modelos3d" :key="m.id" :value="m.id">{{ m.nombre }}</option>
            </select>
          </label>
        </div>
        <label class="radio">
          <input v-model="modeloOpcion" type="radio" value="archivo" />
          <span>{{ $t('modelUpload.uploadNew') }}</span>
        </label>
        <div v-if="modeloOpcion === 'archivo'" class="sub">
          <label class="file-row">
            <span>{{ $t('modelUpload.uploadGlb') }}</span>
            <input type="file" accept=".glb,.gltf,model/gltf-binary,model/gltf+json" @change="onFile" />
          </label>
          <label class="file-row">
            <span>{{ $t('modelUpload.uploadFolder') }}</span>
            <input type="file" webkitdirectory directory multiple @change="onFolder" />
          </label>
          <p v-if="archivo" class="small">
            {{ uploadRelativePath(archivo) }} · {{ Math.round((archivo.size + totalModelBytes(archivoAssets)) / 1024) }} KB
          </p>
        </div>
        <label class="radio">
          <input v-model="modeloOpcion" type="radio" value="ninguno" />
          <span>{{ $t('modelUpload.removeModel') }}</span>
        </label>
        <p v-if="modeloError" class="err">{{ modeloError }}</p>
      </div>
      <template #footer>
        <BaseButton variant="ghost" @click="showModelo = false">{{ $t('common.cancel') }}</BaseButton>
        <BaseButton :loading="modeloBusy" @click="applyModelo">{{ $t('common.save') }}</BaseButton>
      </template>
    </Modal>
  </section>
    </main>
    <AppFooter />
  </div>
</template>

<style scoped>
.head { display: flex; justify-content: space-between; align-items: flex-start; gap: 1rem; margin-bottom: 1rem; flex-wrap: wrap; }
.head h1 { margin: 0; }
.head__sub { margin: 0.35rem 0 0; color: var(--c-text-muted); display: flex; align-items: center; gap: 0.5rem; }
.actions { display: flex; gap: 0.5rem; flex-wrap: wrap; justify-content: flex-end; }
.tabs { display: flex; gap: 0.25rem; margin: 1.25rem 0 0.75rem; border-bottom: 1px solid var(--c-border); }
.tabs button { background: none; border: 0; padding: 0.6rem 0.9rem; cursor: pointer; color: var(--c-text-muted); font: inherit; border-bottom: 2px solid transparent; }
.tabs button.active { color: var(--c-text); border-bottom-color: var(--c-primary); font-weight: 600; }
.panel { background: var(--c-surface); border: 1px solid var(--c-border); border-radius: var(--radius-md); padding: 1rem 1.25rem; }
.def { display: grid; grid-template-columns: 200px 1fr; gap: 0.4rem 1rem; margin: 0; }
.def dt { color: var(--c-text-muted); font-size: 0.9rem; }
.def dd { margin: 0; }
.form { display: flex; flex-direction: column; gap: 0.75rem; }
.radio { display: flex; gap: 0.5rem; align-items: center; cursor: pointer; }
.sub { padding-left: 1.6rem; display: flex; flex-direction: column; gap: 0.4rem; }
.file-row { display: grid; gap: 0.35rem; font-size: 0.875rem; font-weight: 600; }
.sub input[type="file"], .sub input[type="text"] { max-width: 100%; font: inherit; padding: 0.5rem 0.6rem; border: 1px solid var(--c-border); border-radius: var(--radius-md); background: var(--c-surface); color: var(--c-text); }
.small { font-size: 0.85rem; }
code { background: var(--c-surface-2); padding: 0 0.3rem; border-radius: 4px; font-size: 0.85em; }
.select { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.875rem; font-weight: 600; }
.select select, .select textarea { width: 100%; min-width: 0; box-sizing: border-box; padding: 0.55rem 0.65rem; border: 1px solid var(--c-border); border-radius: var(--radius-md); background: var(--c-surface); color: var(--c-text); font: inherit; }
.err { color: var(--c-danger); }
.muted { color: var(--c-text-muted); }
.panel-head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 0.75rem; }
.panel-head h3 { margin: 0; }
.grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0.75rem; }
@media (max-width: 720px) { .grid { grid-template-columns: 1fr; } }
.form-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0.75rem; margin-bottom: 0.75rem; }
@media (max-width: 720px) { .form-grid { grid-template-columns: 1fr; } }
.form-actions { display: flex; justify-content: flex-end; gap: 0.5rem; flex-wrap: wrap; }
.pre { white-space: pre-wrap; }
.mt { margin-top: 1.5rem; }
.timeline { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.85rem; border-left: 2px solid var(--c-border); padding-left: 1rem; }
.timeline li { position: relative; }
.timeline li::before { content: ''; position: absolute; left: -1.4rem; top: 0.55rem; width: 0.6rem; height: 0.6rem; border-radius: 50%; background: var(--c-primary); }
.timeline-head { display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; }
.timeline-head .spacer { margin-left: auto; }
.timeline p { margin: 0.25rem 0 0; }
.changes { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.4rem; }
.changes li { display: flex; flex-wrap: wrap; gap: 0.4rem; align-items: baseline; padding: 0.4rem 0; border-bottom: 1px dashed var(--c-border); }
</style>
