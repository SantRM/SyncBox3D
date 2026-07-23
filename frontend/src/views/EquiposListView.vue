<script setup lang="ts">
import { Plus } from '@lucide/vue'
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'

import AppFooter from '@/components/AppFooter.vue'
import AppNavbar from '@/components/AppNavbar.vue'
import Badge from '@/components/Badge.vue'
import BaseInput from '@/components/BaseInput.vue'
import BaseTable from '@/components/BaseTable.vue'
import RoleGate from '@/components/RoleGate.vue'
import { api } from '@/services/api'
import type { Categoria, Equipo, EstadoOperativo } from '@/services/types'

const { t } = useI18n()

const equipos = ref<Equipo[]>([])
const categorias = ref<Categoria[]>([])
const estados = ref<EstadoOperativo[]>([])
const loading = ref(false)
const errorMsg = ref<string | null>(null)

const q = ref('')
const categoriaId = ref('')
const estadoId = ref('')

const estadoMap = computed(() => new Map(estados.value.map((e) => [e.id, e])))
const categoriaMap = computed(() => new Map(categorias.value.map((c) => [c.id, c])))

let debounce: number | null = null
function scheduleSearch(): void {
  if (debounce !== null) window.clearTimeout(debounce)
  debounce = window.setTimeout(() => void load(), 300)
}

async function load(): Promise<void> {
  loading.value = true
  errorMsg.value = null
  try {
    equipos.value = await api.equipos.list({
      q: q.value || undefined,
      categoria_id: categoriaId.value || undefined,
      estado_id: estadoId.value || undefined,
      limit: 200,
    }) ?? []
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('equipment.loadError')
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  const [c, s] = await Promise.all([api.categorias.list(true), api.estados.list()])
  categorias.value = c
  estados.value = s
  await load()
})

watch([categoriaId, estadoId], () => void load())
watch(q, scheduleSearch)

const cols = computed(() => [
  { key: 'nombre', label: t('common.name') },
  { key: 'fabricante', label: t('common.manufacturer') },
  { key: 'modelo', label: t('common.model') },
  { key: 'serial', label: t('common.serial') },
  { key: 'categoria', label: t('common.category') },
  { key: 'estado', label: t('common.status') },
  { key: 'acciones', label: '' },
])
</script>

<template>
  <div class="app-shell">
    <AppNavbar />
    <main class="page">
  <section>
    <header class="head">
      <h1>{{ $t('equipment.title') }}</h1>
      <RoleGate :roles="['ADMINISTRADOR', 'OPERADOR']">
        <RouterLink :to="{ name: 'ubicaciones' }" class="create-link">
          <Plus :size="16" aria-hidden="true" /> {{ $t('equipment.createFromLocations') }}
        </RouterLink>
      </RoleGate>
    </header>

    <div class="filters">
      <BaseInput v-model="q" :placeholder="$t('equipment.searchPlaceholder')" type="search" :label="$t('common.search')" />
      <label class="select">
        <span>{{ $t('common.category') }}</span>
        <select v-model="categoriaId">
          <option value="">{{ $t('common.all') }}</option>
          <option v-for="c in categorias" :key="c.id" :value="c.id">{{ c.nombre }}</option>
        </select>
      </label>
      <label class="select">
        <span>{{ $t('common.status') }}</span>
        <select v-model="estadoId">
          <option value="">{{ $t('common.allMasc') }}</option>
          <option v-for="e in estados" :key="e.id" :value="e.id">{{ e.nombre }}</option>
        </select>
      </label>
    </div>

    <p v-if="errorMsg" class="err">{{ errorMsg }}</p>
    <p v-if="loading" class="muted">{{ $t('common.loading') }}</p>

    <BaseTable :rows="equipos" :columns="cols" :row-key="(r) => r.id" :empty="$t('equipment.empty')">
      <template #cell-categoria="{ row }">
        {{ categoriaMap.get((row as Equipo).categoria_id)?.nombre ?? '—' }}
      </template>
      <template #cell-estado="{ row }">
        <Badge :color="estadoMap.get((row as Equipo).estado_id)?.color">
          {{ estadoMap.get((row as Equipo).estado_id)?.nombre ?? '—' }}
        </Badge>
      </template>
      <template #cell-acciones="{ row }">
        <RouterLink :to="{ name: 'equipo-detalle', params: { id: (row as Equipo).id } }">{{ $t('equipment.view') }}</RouterLink>
      </template>
    </BaseTable>

  </section>
    </main>
    <AppFooter />
  </div>
</template>

<style scoped>
.head { display: flex; justify-content: space-between; align-items: center; margin-bottom: 1rem; }
.head h1 { margin: 0; }
.create-link {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  padding: 0.55rem 0.8rem;
  border-radius: var(--radius-md);
  background: var(--c-primary);
  color: #fff;
  font-weight: 700;
  text-decoration: none;
}
.filters { display: grid; gap: 0.75rem; grid-template-columns: minmax(260px, 2fr) minmax(180px, 1fr) minmax(180px, 1fr); align-items: end; margin-bottom: 1rem; }
.select { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.875rem; font-weight: 600; }
.select select {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
  padding: 0.55rem 0.5rem;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  background: var(--c-surface); color: var(--c-text);
  font: inherit;
}
.err { color: var(--c-danger); }
.muted { color: var(--c-text-muted); }
@media (max-width: 720px) { .filters { grid-template-columns: 1fr; } }
</style>
