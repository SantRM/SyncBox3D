<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'

import { Clock, RotateCcw, Search, ShieldCheck } from '@lucide/vue'

import AppFooter from '@/components/AppFooter.vue'
import AppNavbar from '@/components/AppNavbar.vue'
import BaseButton from '@/components/BaseButton.vue'
import BaseInput from '@/components/BaseInput.vue'
import { api } from '@/services/api'
import { dateLocale } from '@/i18n'
import type { AlertaConfig, AlertaEvento } from '@/services/types'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const { locale, t } = useI18n()

const alertas = ref<AlertaEvento[]>([])
const configs = ref<AlertaConfig[]>([])
const errorMsg = ref<string | null>(null)
const configMsg = ref<string | null>(null)
const loading = ref(false)
const loadingConfig = ref(false)
const savingConfig = ref<string | null>(null)

const estadoFiltro = ref<'pendiente' | 'resuelta' | ''>('pendiente')
const q = ref('')
const showConfig = ref(false)

const canConfig = computed(() => auth.hasRole('ADMINISTRADOR'))
const pendingCount = computed(() => alertas.value.filter((a) => !a.resuelta_at).length)

function formatDate(value?: string | null): string {
  if (!value) return t('alerts.pending')
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(dateLocale(locale.value), {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

function alertaRazon(a: AlertaEvento): string {
  if (a.razon) return a.razon
  if (a.dias_umbral && a.dias_en_estado !== undefined && a.estado_nombre && a.equipo_nombre) {
    return t('alerts.reasonThreshold', {
      equipment: a.equipo_nombre,
      days: a.dias_en_estado,
      status: a.estado_nombre,
      threshold: a.dias_umbral,
    })
  }
  if (a.estado_nombre && a.equipo_nombre) return t('alerts.reasonStatus', { equipment: a.equipo_nombre, status: a.estado_nombre })
  return t('alerts.reasonDefault')
}

async function load(): Promise<void> {
  loading.value = true
  errorMsg.value = null
  try {
    alertas.value = await api.alertas.list({
      estado: estadoFiltro.value,
      q: q.value.trim() || undefined,
      limit: 200,
    })
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('alerts.loadError')
  } finally {
    loading.value = false
  }
}

async function loadConfig(): Promise<void> {
  if (!canConfig.value) return
  loadingConfig.value = true
  configMsg.value = null
  try {
    configs.value = await api.alertas.config()
  } catch (e) {
    configMsg.value = (e as { message?: string }).message ?? t('alerts.configLoadError')
  } finally {
    loadingConfig.value = false
  }
}

async function applyFilters(): Promise<void> {
  await load()
}

async function clearFilters(): Promise<void> {
  estadoFiltro.value = 'pendiente'
  q.value = ''
  await load()
}

async function resolver(a: AlertaEvento): Promise<void> {
  try {
    await api.alertas.resolver(a.id)
    await load()
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('alerts.resolveError')
  }
}

async function posponer(a: AlertaEvento): Promise<void> {
  try {
    await api.alertas.posponer(a.id, 60)
    await load()
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('alerts.snoozeError')
  }
}

async function saveConfig(c: AlertaConfig): Promise<void> {
  if (c.protegida) return
  savingConfig.value = c.estado_id
  configMsg.value = null
  try {
    const saved = await api.alertas.updateConfig(c.estado_id, {
      dias_umbral: Math.max(1, Number(c.dias_umbral) || 1),
      activa: c.activa,
    })
    configs.value = configs.value.map((row) => row.estado_id === saved.estado_id ? saved : row)
    await load()
  } catch (e) {
    configMsg.value = (e as { message?: string }).message ?? t('alerts.configError')
  } finally {
    savingConfig.value = null
  }
}

function stateLabel(a: AlertaEvento): string {
  if (a.resuelta_at) return t('alerts.resolved')
  if (a.pospuesta_hasta && new Date(a.pospuesta_hasta).getTime() > Date.now()) return t('alerts.snoozed')
  return t('alerts.pending')
}

onMounted(async () => {
  await Promise.all([load(), loadConfig()])
})
</script>

<template>
  <div class="app-shell">
    <AppNavbar />
    <main class="alerts-page">
      <header class="alerts-head">
        <div>
          <p>{{ $t('alerts.operation') }}</p>
          <h1>{{ $t('alerts.title') }}</h1>
        </div>
        <BaseButton v-if="canConfig" variant="secondary" @click="showConfig = !showConfig">
          {{ showConfig ? $t('alerts.hideConfig') : $t('alerts.configureThresholds') }}
        </BaseButton>
      </header>

      <section v-if="showConfig && canConfig" class="config-panel">
        <div class="section-head">
          <div>
            <h2>{{ $t('alerts.thresholds') }}</h2>
            <p>{{ $t('alerts.protectedHint') }}</p>
          </div>
        </div>
        <p v-if="configMsg" class="err" role="alert">{{ configMsg }}</p>
        <p v-if="loadingConfig" class="muted">{{ $t('alerts.loadingConfig') }}</p>
        <div v-else class="config-list">
          <article v-for="cfg in configs" :key="cfg.estado_id" class="config-item">
            <span class="state-dot" :style="{ background: cfg.estado_color || '#64748b' }" />
            <div class="config-main">
              <strong>{{ cfg.estado_nombre }}</strong>
              <small v-if="cfg.protegida">{{ $t('alerts.protectedState') }}</small>
              <small v-else>{{ $t('alerts.thresholdState') }}</small>
            </div>
            <label class="switch">
              <span>{{ $t('common.enabled') }}</span>
              <input v-model="cfg.activa" type="checkbox" :disabled="cfg.protegida" @change="saveConfig(cfg)" />
            </label>
            <label class="days">
              <span>{{ $t('alerts.days') }}</span>
              <input
                v-model.number="cfg.dias_umbral"
                type="number"
                min="1"
                step="1"
                inputmode="numeric"
                :disabled="cfg.protegida || !cfg.activa"
                @change="saveConfig(cfg)"
              />
            </label>
            <span v-if="savingConfig === cfg.estado_id" class="saving">{{ $t('common.saving') }}</span>
          </article>
        </div>
      </section>

      <section class="alerts-toolbar">
        <form class="filters" @submit.prevent="applyFilters">
          <BaseInput v-model="q" type="search" :label="$t('common.search')" :placeholder="$t('alerts.searchPlaceholder')" />
          <label class="field">
            <span>{{ $t('alerts.alertStatus') }}</span>
            <select v-model="estadoFiltro">
              <option value="pendiente">{{ $t('alerts.pending') }}</option>
              <option value="resuelta">{{ $t('alerts.resolved') }}</option>
              <option value="">{{ $t('alerts.all') }}</option>
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

      <p v-if="errorMsg" class="err" role="alert">{{ errorMsg }}</p>
      <p v-if="loading" class="muted">{{ $t('common.loading') }}</p>

      <section class="alerts-table-card">
        <div class="table-head">
          <strong>{{ $t('alerts.summary', { count: alertas.length }) }}</strong>
          <small>{{ $t('alerts.pendingVisible', { count: pendingCount }) }}</small>
        </div>
        <div class="table-wrap">
          <table class="alerts-table">
            <thead>
              <tr>
                <th>{{ $t('alerts.equipment') }}</th>
                <th>{{ $t('common.status') }}</th>
                <th>{{ $t('alerts.situation') }}</th>
                <th>{{ $t('alerts.dates') }}</th>
                <th>{{ $t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-if="!loading && alertas.length === 0">
                <td colspan="5" class="empty">{{ $t('alerts.emptyFiltered') }}</td>
              </tr>
              <tr v-for="a in alertas" :key="a.id">
                <td>
                  <RouterLink :to="{ name: 'equipo-detalle', params: { id: a.equipo_id } }">
                    {{ a.equipo_nombre || `${a.equipo_id.slice(0, 8)}...` }}
                  </RouterLink>
                  <small>{{ a.equipo_id.slice(0, 8) }}</small>
                </td>
                <td>
                  <span class="state-line">
                    <span class="state-dot" :style="{ background: a.estado_color || '#64748b' }" />
                    {{ a.estado_nombre || a.estado_id.slice(0, 8) }}
                  </span>
                  <small>{{ $t('alerts.daysInState', { count: a.dias_en_estado ?? 0 }) }}</small>
                </td>
                <td>
                  <span class="status-chip" :class="{ done: !!a.resuelta_at, snoozed: !!a.pospuesta_hasta && !a.resuelta_at }">
                    {{ stateLabel(a) }}
                  </span>
                  <p class="reason">{{ alertaRazon(a) }}</p>
                </td>
                <td>
                  <dl class="dates">
                    <div>
                      <dt>{{ $t('alerts.generatedAt') }}</dt>
                      <dd>{{ formatDate(a.generada_at) }}</dd>
                    </div>
                    <div>
                      <dt>{{ $t('alerts.resolvedAt') }}</dt>
                      <dd>{{ formatDate(a.resuelta_at) }}</dd>
                    </div>
                    <div v-if="a.pospuesta_hasta && !a.resuelta_at">
                      <dt>{{ $t('alerts.repeatsAt') }}</dt>
                      <dd>{{ formatDate(a.pospuesta_hasta) }}</dd>
                    </div>
                  </dl>
                </td>
                <td>
                  <div v-if="!a.resuelta_at" class="row-actions">
                    <button class="link" type="button" @click="posponer(a)">
                      <Clock :size="15" aria-hidden="true" /> {{ $t('alerts.snooze') }}
                    </button>
                    <button class="link" type="button" @click="resolver(a)">
                      <ShieldCheck :size="15" aria-hidden="true" /> {{ $t('alerts.resolve') }}
                    </button>
                  </div>
                  <span v-else class="muted">{{ a.resolucion_motivo || 'manual' }}</span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>
    </main>
    <AppFooter />
  </div>
</template>

<style scoped>
.alerts-page {
  width: min(1500px, calc(100vw - 1.5rem));
  margin: 0 auto;
  padding: 0.9rem 0;
}

.alerts-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.85rem;
}

.alerts-head p,
.alerts-head h1 {
  margin: 0;
}

.alerts-head p {
  color: var(--c-text-muted);
  font-size: 0.82rem;
  font-weight: 800;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.alerts-head h1 {
  font-size: 1.35rem;
}

.config-panel,
.alerts-toolbar,
.alerts-table-card {
  background: var(--c-surface);
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
}

.config-panel {
  padding: 0.85rem;
  margin-bottom: 0.8rem;
}

.section-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  margin-bottom: 0.7rem;
}

.section-head h2,
.section-head p {
  margin: 0;
}

.section-head h2 {
  font-size: 1rem;
}

.section-head p {
  color: var(--c-text-muted);
  font-size: 0.86rem;
}

.config-list {
  display: grid;
  gap: 0.55rem;
}

.config-item {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto auto auto;
  align-items: center;
  gap: 0.75rem;
  padding: 0.65rem;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-sm);
  background: var(--c-surface-2);
}

.config-main {
  display: flex;
  flex-direction: column;
  gap: 0.12rem;
  min-width: 0;
}

.config-main small,
.muted,
.alerts-table small {
  color: var(--c-text-muted);
}

.switch,
.days {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  color: var(--c-text-muted);
  font-size: 0.84rem;
}

.switch input {
  width: 18px;
  height: 18px;
  accent-color: var(--c-primary);
}

.days input {
  width: 78px;
  min-height: 34px;
  padding: 0.35rem 0.45rem;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-sm);
  background: var(--c-surface);
  color: var(--c-text);
  font: inherit;
}

.saving {
  color: var(--c-text-muted);
  font-size: 0.82rem;
}

.alerts-toolbar {
  padding: 0.8rem;
  margin-bottom: 0.8rem;
}

.filters {
  display: grid;
  grid-template-columns: minmax(260px, 1fr) minmax(170px, 0.35fr) auto;
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

.field select {
  min-height: 40px;
  padding: 0.5rem 0.65rem;
  border: 1px solid var(--c-border);
  border-radius: var(--radius-md);
  background: var(--c-surface);
  color: var(--c-text);
  font: inherit;
}

.filter-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 0.45rem;
}

.err {
  margin: 0 0 0.8rem;
  color: var(--c-danger);
}

.table-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  padding: 0.8rem 0.9rem;
  border-bottom: 1px solid var(--c-border);
}

.table-wrap {
  overflow-x: auto;
}

.alerts-table {
  width: 100%;
  min-width: 1080px;
  border-collapse: collapse;
  font-size: 0.9rem;
}

.alerts-table th,
.alerts-table td {
  padding: 0.75rem 0.85rem;
  border-bottom: 1px solid var(--c-border);
  text-align: left;
  vertical-align: top;
}

.alerts-table th {
  background: var(--c-surface-2);
  color: var(--c-text-muted);
  font-size: 0.78rem;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.alerts-table td > a,
.alerts-table td > small {
  display: block;
}

.empty {
  padding: 1.5rem !important;
  color: var(--c-text-muted);
  text-align: center !important;
}

.state-line {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  font-weight: 700;
}

.state-dot {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.status-chip {
  display: inline-flex;
  align-items: center;
  min-height: 24px;
  padding: 0.16rem 0.5rem;
  border-radius: 999px;
  background: #fef3c7;
  color: #92400e;
  font-size: 0.76rem;
  font-weight: 800;
}

.status-chip.snoozed {
  background: #e0f2fe;
  color: #075985;
}

.status-chip.done {
  background: #dcfce7;
  color: #166534;
}

.reason {
  max-width: 520px;
  margin: 0.35rem 0 0;
  line-height: 1.35;
}

.dates {
  display: grid;
  gap: 0.32rem;
  margin: 0;
}

.dates div {
  display: grid;
  grid-template-columns: 92px minmax(0, 1fr);
  gap: 0.45rem;
}

.dates dt {
  color: var(--c-text-muted);
}

.dates dd {
  margin: 0;
  font-weight: 700;
}

.row-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
}

.link {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  background: none;
  border: 0;
  color: var(--c-primary);
  cursor: pointer;
  font: inherit;
  font-weight: 700;
  padding: 0;
}

.link:hover {
  text-decoration: underline;
}

@media (max-width: 820px) {
  .alerts-head,
  .table-head,
  .filter-actions {
    align-items: stretch;
    flex-direction: column;
  }

  .filters,
  .config-item {
    grid-template-columns: 1fr;
  }

  .state-dot {
    justify-self: start;
  }
}
</style>
