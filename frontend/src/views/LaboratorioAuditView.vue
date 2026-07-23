<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'

import { ArrowLeft, RotateCcw, Search } from '@lucide/vue'

import AppFooter from '@/components/AppFooter.vue'
import AppNavbar from '@/components/AppNavbar.vue'
import BaseButton from '@/components/BaseButton.vue'
import BaseInput from '@/components/BaseInput.vue'
import { api } from '@/services/api'
import { dateLocale } from '@/i18n'
import type { EscenaDetail, LabAuditEntry } from '@/services/types'

const props = defineProps<{ id: string }>()

const route = useRoute()
const { locale, t } = useI18n()

const escena = ref<EscenaDetail | null>(null)
const rows = ref<LabAuditEntry[]>([])
const loading = ref(false)
const loadingEscena = ref(false)
const errorMsg = ref<string | null>(null)

const q = ref('')
const desde = ref('')
const hasta = ref('')
const estado = ref('')
const limit = ref(50)
const offset = ref(0)
const total = ref(0)

const labRoute = computed(() => ({
  name: 'laboratorio-detalle',
  params: { id: props.id },
  query: route.query,
}))

const pageStart = computed(() => total.value === 0 ? 0 : offset.value + 1)
const pageEnd = computed(() => Math.min(offset.value + limit.value, total.value))
const canPrev = computed(() => offset.value > 0)
const canNext = computed(() => offset.value + limit.value < total.value)

function auditParams() {
  return {
    q: q.value.trim() || undefined,
    desde: desde.value || undefined,
    hasta: hasta.value || undefined,
    estado: estado.value || undefined,
    limit: limit.value,
    offset: offset.value,
  }
}

async function loadEscena(): Promise<void> {
  loadingEscena.value = true
  try {
    escena.value = await api.escenas.get(props.id)
  } catch {
    escena.value = null
  } finally {
    loadingEscena.value = false
  }
}

async function loadAudit(): Promise<void> {
  loading.value = true
  errorMsg.value = null
  try {
    const res = await api.escenas.auditoria(props.id, auditParams())
    rows.value = res.items ?? []
    total.value = res.total ?? 0
    limit.value = res.limit ?? limit.value
    offset.value = res.offset ?? offset.value
  } catch (e) {
    rows.value = []
    total.value = 0
    errorMsg.value = (e as { message?: string }).message ?? t('audit.loadError')
  } finally {
    loading.value = false
  }
}

async function applyFilters(): Promise<void> {
  offset.value = 0
  await loadAudit()
}

async function clearFilters(): Promise<void> {
  q.value = ''
  desde.value = ''
  hasta.value = ''
  estado.value = ''
  offset.value = 0
  await loadAudit()
}

async function prevPage(): Promise<void> {
  if (!canPrev.value) return
  offset.value = Math.max(0, offset.value - limit.value)
  await loadAudit()
}

async function nextPage(): Promise<void> {
  if (!canNext.value) return
  offset.value += limit.value
  await loadAudit()
}

function formatDate(value?: string | null): string {
  if (!value) return t('audit.open')
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(dateLocale(locale.value), {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

function shortId(id?: string): string {
  if (!id) return t('audit.noSession')
  return id.slice(0, 8)
}

function fixed(value: number, digits = 2): string {
  return value.toFixed(digits)
}

function degrees(value: number): string {
  return `${(value * 180 / Math.PI).toFixed(0)}°`
}

function objectSubtitle(row: LabAuditEntry): string {
  const parts = [row.fabricante_snapshot, row.modelo_snapshot, row.categoria_snapshot]
    .map((v) => v?.trim())
    .filter(Boolean)
  return parts.length ? parts.join(' · ') : t('audit.instance', { id: shortId(row.instancia_id) })
}

function userLabel(row: LabAuditEntry): string {
  return row.usuario_nombre || row.usuario_correo || t('audit.userUnavailable')
}

function eventLabel(row: LabAuditEntry): string {
  const labels: Record<LabAuditEntry['event_type'], string> = {
    add: t('audit.actions.add'),
    transform: t('audit.actions.transform'),
    restore: t('audit.actions.restore'),
    restore_session: t('audit.actions.restore_session'),
    remove: t('audit.actions.remove'),
  }
  return labels[row.event_type] ?? row.event_type
}

watch(() => props.id, async () => {
  offset.value = 0
  await Promise.all([loadEscena(), loadAudit()])
})

onMounted(async () => {
  await Promise.all([loadEscena(), loadAudit()])
})
</script>

<template>
  <div>
    <AppNavbar />
    <main class="audit-page">
      <header class="audit-head">
        <RouterLink class="back" :to="labRoute">
          <ArrowLeft :size="17" aria-hidden="true" /> {{ $t('nodeTypes.LABORATORIO') }}
        </RouterLink>
        <div>
          <p>{{ $t('audit.title') }}</p>
          <h1>{{ escena?.nombre || (loadingEscena ? $t('common.loading') : $t('nodeTypes.LABORATORIO')) }}</h1>
        </div>
      </header>

      <section class="audit-toolbar" :aria-label="$t('audit.filtersAria')">
        <form class="filters" @submit.prevent="applyFilters">
          <BaseInput
            v-model="q"
            type="search"
            :label="$t('common.search')"
            :placeholder="$t('audit.searchPlaceholder')"
          />

          <label class="field">
            <span>{{ $t('common.from') }}</span>
            <input v-model="desde" type="date" />
          </label>

          <label class="field">
            <span>{{ $t('common.to') }}</span>
            <input v-model="hasta" type="date" />
          </label>

          <label class="field">
            <span>{{ $t('common.session') }}</span>
            <select v-model="estado">
              <option value="">{{ $t('audit.allSessions') }}</option>
              <option value="abierta">{{ $t('audit.openSessions') }}</option>
              <option value="cerrada">{{ $t('audit.closedSessions') }}</option>
            </select>
          </label>

          <div class="filter-actions">
            <BaseButton type="submit" :loading="loading">
              <Search :size="16" aria-hidden="true" /> {{ $t('common.filter') }}
            </BaseButton>
            <BaseButton variant="ghost" @click="clearFilters">
              <RotateCcw :size="16" aria-hidden="true" /> {{ $t('common.clear') }}
            </BaseButton>
          </div>
        </form>
      </section>

      <p v-if="errorMsg" class="error" role="alert">{{ errorMsg }}</p>

      <section class="audit-table-card" :aria-label="$t('audit.recordsAria')">
        <div class="table-head">
          <div>
            <strong>{{ $t('audit.total', { count: total }) }}</strong>
            <small v-if="total > 0">{{ $t('audit.showing', { start: pageStart, end: pageEnd }) }}</small>
          </div>
          <div class="pager">
            <BaseButton variant="secondary" :disabled="!canPrev || loading" @click="prevPage">
              {{ $t('audit.previous') }}
            </BaseButton>
            <BaseButton variant="secondary" :disabled="!canNext || loading" @click="nextPage">
              {{ $t('audit.next') }}
            </BaseButton>
          </div>
        </div>

        <div class="table-wrap">
          <table class="audit-table">
            <thead>
              <tr>
                <th>{{ $t('audit.action') }}</th>
                <th>{{ $t('common.date') }}</th>
                <th>{{ $t('audit.object') }}</th>
                <th>{{ $t('common.user') }}</th>
                <th>{{ $t('common.session') }}</th>
                <th>{{ $t('audit.transform') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="loading">
                <td colspan="6" class="empty">{{ $t('audit.loading') }}</td>
              </tr>
              <tr v-else-if="rows.length === 0">
                <td colspan="6" class="empty">{{ $t('audit.empty') }}</td>
              </tr>
              <template v-else>
                <tr v-for="row in rows" :key="`${row.fecha}:${row.lab_sesion_id || 'sin-sesion'}:${row.instancia_id}`">
                  <td>
                    <span class="event-chip" :class="`event-chip--${row.event_type}`">
                      {{ eventLabel(row) }}
                    </span>
                  </td>
                  <td>
                    <strong>{{ formatDate(row.fecha) }}</strong>
                    <small>{{ $t('audit.activity') }}: {{ formatDate(row.sesion_ultima_actividad_at) }}</small>
                  </td>
                  <td>
                    <strong>{{ row.nombre_snapshot || $t('audit.unnamedObject') }}</strong>
                    <small>{{ objectSubtitle(row) }}</small>
                  </td>
                  <td>
                    <strong>{{ userLabel(row) }}</strong>
                    <small>{{ row.usuario_correo || $t('audit.noEmail') }}</small>
                  </td>
                  <td>
                    <span class="status-chip" :class="{ open: !row.sesion_cerrada_at }">
                      {{ row.sesion_cerrada_at ? $t('audit.closed') : $t('audit.open') }}
                    </span>
                    <small>{{ shortId(row.lab_sesion_id) }} · {{ $t('audit.startedAt') }} {{ formatDate(row.sesion_iniciada_at) }}</small>
                  </td>
                  <td>
                    <div class="transform-grid">
                      <span>X {{ fixed(row.pos_x) }}</span>
                      <span>Y {{ fixed(row.pos_y) }}</span>
                      <span>Z {{ fixed(row.pos_z) }}</span>
                      <span>Esc {{ fixed(row.escala) }}</span>
                      <span>RX {{ degrees(row.rot_x) }}</span>
                      <span>RY {{ degrees(row.rot_y) }}</span>
                      <span>RZ {{ degrees(row.rot_z) }}</span>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
      </section>
    </main>
    <AppFooter />
  </div>
</template>

<style scoped>
.audit-page {
  width: min(1500px, calc(100vw - 1.5rem));
  margin: 0 auto;
  padding: 0.9rem 0;
}

.audit-head {
  display: flex;
  align-items: center;
  gap: 1rem;
  margin-bottom: 0.85rem;
}

.audit-head h1,
.audit-head p {
  margin: 0;
}

.audit-head p {
  color: var(--c-text-muted);
  font-size: 0.82rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.audit-head h1 {
  font-size: 1.35rem;
}

.back {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: var(--c-text-muted);
  text-decoration: none;
}

.back:hover {
  color: var(--c-text);
}

.audit-toolbar,
.audit-table-card {
  background: var(--c-surface);
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
}

.audit-toolbar {
  padding: 0.8rem;
  margin-bottom: 0.8rem;
}

.filters {
  display: grid;
  grid-template-columns: minmax(260px, 1.4fr) repeat(3, minmax(150px, 0.65fr)) auto;
  align-items: end;
  gap: 0.7rem;
}

.field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}

.field span {
  font-size: 0.875rem;
  font-weight: 600;
}

.field input,
.field select {
  min-height: 40px;
  padding: 0.5rem 0.65rem;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  background: var(--c-surface);
  color: var(--c-text);
  font: inherit;
}

.field input:focus,
.field select:focus {
  outline: 2px solid var(--c-focus);
  outline-offset: 1px;
  border-color: var(--c-primary);
}

.filter-actions {
  display: flex;
  gap: 0.45rem;
  align-items: center;
  justify-content: flex-end;
}

.error {
  margin: 0 0 0.8rem;
  padding: 0.7rem 0.85rem;
  border: 1px solid #fecaca;
  border-radius: var(--radius-md);
  background: #fff1f2;
  color: #991b1b;
}

.audit-table-card {
  overflow: hidden;
}

.table-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.8rem 0.9rem;
  border-bottom: 1px solid var(--c-border);
}

.table-head div:first-child {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
}

.table-head small,
.audit-table small {
  color: var(--c-text-muted);
  font-size: 0.78rem;
}

.pager {
  display: flex;
  gap: 0.45rem;
}

.table-wrap {
  overflow-x: auto;
}

.audit-table {
  width: 100%;
  min-width: 1120px;
  border-collapse: collapse;
  font-size: 0.9rem;
}

.audit-table th,
.audit-table td {
  padding: 0.75rem 0.85rem;
  border-bottom: 1px solid var(--c-border);
  text-align: left;
  vertical-align: top;
}

.audit-table th {
  background: var(--c-surface-2);
  color: var(--c-text-muted);
  font-size: 0.78rem;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.audit-table td {
  background: var(--c-surface);
}

.audit-table tr:last-child td {
  border-bottom: 0;
}

.audit-table td > strong,
.audit-table td > small {
  display: block;
}

.empty {
  padding: 1.6rem !important;
  color: var(--c-text-muted);
  text-align: center !important;
}

.status-chip {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  margin-bottom: 0.3rem;
  padding: 0.18rem 0.5rem;
  border-radius: 999px;
  background: #eef2ff;
  color: #3730a3;
  font-size: 0.76rem;
  font-weight: 800;
}

.status-chip.open {
  background: #dcfce7;
  color: #166534;
}

.event-chip {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0.18rem 0.5rem;
  border-radius: var(--radius-sm);
  background: #e2e8f0;
  color: #334155;
  font-size: 0.76rem;
  font-weight: 800;
}

.event-chip--add {
  background: #dcfce7;
  color: #166534;
}

.event-chip--remove {
  background: #fee2e2;
  color: #991b1b;
}

.event-chip--restore,
.event-chip--restore_session {
  background: #e0f2fe;
  color: #075985;
}

.transform-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(72px, 1fr));
  gap: 0.35rem;
}

.transform-grid span {
  display: inline-flex;
  justify-content: space-between;
  min-height: 28px;
  padding: 0.25rem 0.45rem;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-sm);
  background: var(--c-surface-2);
  color: var(--c-text);
  font-size: 0.78rem;
  font-variant-numeric: tabular-nums;
}

@media (max-width: 1180px) {
  .filters {
    grid-template-columns: 1fr 1fr;
  }

  .filter-actions {
    justify-content: flex-start;
  }
}

@media (max-width: 680px) {
  .audit-head,
  .table-head,
  .filter-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .filters {
    grid-template-columns: 1fr;
  }

  .pager {
    width: 100%;
  }
}
</style>
