<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'

import { Clock, ShieldCheck } from '@lucide/vue'

import { api } from '@/services/api'
import { dateLocale } from '@/i18n'
import type { AlertaEvento } from '@/services/types'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const { locale, t } = useI18n()
const alerts = ref<AlertaEvento[]>([])
const loading = ref(false)
const acting = ref(false)
const errorMsg = ref<string | null>(null)
let timer: number | null = null

const canHandleAlerts = computed(() => auth.hasRole('ADMINISTRADOR', 'OPERADOR'))
const current = computed(() => alerts.value[0] ?? null)
const extraCount = computed(() => Math.max(alerts.value.length - 1, 0))

function formatDate(value?: string | null): string {
  if (!value) return ''
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat(dateLocale(locale.value), {
    dateStyle: 'medium',
    timeStyle: 'short',
  }).format(date)
}

function reason(a: AlertaEvento): string {
  if (a.razon) return a.razon
  if (a.equipo_nombre && a.estado_nombre) {
    return t('alerts.popup.reason', { equipment: a.equipo_nombre, status: a.estado_nombre })
  }
  return t('alerts.popup.pendingReason')
}

async function load(): Promise<void> {
  if (!canHandleAlerts.value || loading.value || acting.value) return
  loading.value = true
  errorMsg.value = null
  try {
    alerts.value = await api.alertas.pendientes(true)
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('alerts.popup.loadError')
  } finally {
    loading.value = false
  }
}

async function resolveCurrent(): Promise<void> {
  const alert = current.value
  if (!alert || acting.value) return
  acting.value = true
  errorMsg.value = null
  try {
    await api.alertas.resolver(alert.id)
    alerts.value = alerts.value.filter((a) => a.id !== alert.id)
    void load()
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('alerts.popup.resolveError')
  } finally {
    acting.value = false
  }
}

async function snoozeCurrent(): Promise<void> {
  const alert = current.value
  if (!alert || acting.value) return
  acting.value = true
  errorMsg.value = null
  try {
    await api.alertas.posponer(alert.id, 60)
    alerts.value = alerts.value.filter((a) => a.id !== alert.id)
    void load()
  } catch (e) {
    errorMsg.value = (e as { message?: string }).message ?? t('alerts.popup.snoozeError')
  } finally {
    acting.value = false
  }
}

function startPolling(): void {
  stopPolling()
  if (!canHandleAlerts.value) return
  void load()
  timer = window.setInterval(() => void load(), 60_000)
}

function stopPolling(): void {
  if (timer !== null) window.clearInterval(timer)
  timer = null
}

watch(canHandleAlerts, (enabled) => {
  if (enabled) startPolling()
  else {
    stopPolling()
    alerts.value = []
  }
}, { immediate: true })

onBeforeUnmount(stopPolling)
</script>

<template>
  <Teleport to="body">
    <div v-if="current" class="alert-modal" role="presentation">
      <section class="alert-card" role="dialog" aria-modal="true" aria-labelledby="active-alert-title">
        <header class="alert-card__head">
          <span class="alert-card__icon" aria-hidden="true">!</span>
          <div>
            <p>{{ $t('alerts.popup.title') }}</p>
            <h2 id="active-alert-title">{{ current.equipo_nombre || $t('alerts.popup.fallbackEquipment') }}</h2>
          </div>
        </header>

        <p class="alert-card__reason">{{ reason(current) }}</p>

        <dl class="alert-card__meta">
          <div>
            <dt>{{ $t('common.status') }}</dt>
            <dd>
              <span class="state-dot" :style="{ background: current.estado_color || '#64748b' }" />
              {{ current.estado_nombre || $t('alerts.popup.noStatus') }}
            </dd>
          </div>
          <div>
            <dt>{{ $t('common.generatedAt') }}</dt>
            <dd>{{ formatDate(current.generada_at) }}</dd>
          </div>
          <div v-if="current.pospuesta_hasta">
            <dt>{{ $t('alerts.repeatsAt') }}</dt>
            <dd>{{ formatDate(current.pospuesta_hasta) }}</dd>
          </div>
        </dl>

        <p v-if="extraCount" class="alert-card__more">
          {{ $t('alerts.popup.extra', { count: extraCount }) }}
        </p>
        <p v-if="errorMsg" class="alert-card__error" role="alert">{{ errorMsg }}</p>

        <footer class="alert-card__actions">
          <RouterLink class="alert-link" :to="{ name: 'alertas' }">{{ $t('alerts.popup.viewAlerts') }}</RouterLink>
          <button class="alert-btn alert-btn--secondary" type="button" :disabled="acting" @click="snoozeCurrent">
            <Clock :size="16" aria-hidden="true" /> {{ $t('alerts.popup.snooze') }}
          </button>
          <button class="alert-btn alert-btn--primary" type="button" :disabled="acting" @click="resolveCurrent">
            <ShieldCheck :size="16" aria-hidden="true" /> {{ $t('alerts.popup.resolve') }}
          </button>
        </footer>
      </section>
    </div>
  </Teleport>
</template>

<style scoped>
.alert-modal {
  position: fixed;
  inset: 0;
  z-index: 3000;
  display: grid;
  place-items: center;
  padding: 1rem;
  background: rgba(15, 23, 42, 0.48);
  backdrop-filter: blur(5px);
}

.alert-card {
  width: min(520px, 100%);
  border: 1px solid rgba(148, 163, 184, 0.45);
  border-radius: var(--radius-md);
  background: var(--c-surface);
  color: var(--c-text);
  box-shadow: 0 24px 80px rgba(15, 23, 42, 0.36);
  overflow: hidden;
}

.alert-card__head {
  display: flex;
  align-items: center;
  gap: 0.85rem;
  padding: 1rem 1.1rem;
  border-bottom: 1px solid var(--c-border);
  background: var(--c-surface-2);
}

.alert-card__icon {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  background: #fef3c7;
  color: #92400e;
  font-weight: 900;
  font-size: 1.2rem;
}

.alert-card__head p,
.alert-card__head h2 {
  margin: 0;
}

.alert-card__head p {
  color: var(--c-text-muted);
  font-size: 0.78rem;
  font-weight: 800;
  letter-spacing: 0.05em;
  text-transform: uppercase;
}

.alert-card__head h2 {
  font-size: 1.15rem;
}

.alert-card__reason {
  margin: 0;
  padding: 1rem 1.1rem 0.4rem;
  line-height: 1.45;
}

.alert-card__meta {
  display: grid;
  gap: 0.45rem;
  margin: 0;
  padding: 0.5rem 1.1rem 1rem;
}

.alert-card__meta div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  min-height: 30px;
}

.alert-card__meta dt {
  color: var(--c-text-muted);
  font-size: 0.82rem;
}

.alert-card__meta dd {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  margin: 0;
  font-weight: 700;
  text-align: right;
}

.state-dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
}

.alert-card__more,
.alert-card__error {
  margin: 0 1.1rem 0.8rem;
  font-size: 0.88rem;
}

.alert-card__more {
  color: var(--c-text-muted);
}

.alert-card__error {
  color: var(--c-danger);
}

.alert-card__actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 0.5rem;
  padding: 0.9rem 1.1rem 1rem;
  border-top: 1px solid var(--c-border);
}

.alert-link,
.alert-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  min-height: 38px;
  padding: 0.5rem 0.8rem;
  border-radius: var(--radius-sm);
  font: inherit;
  font-weight: 700;
  text-decoration: none;
}

.alert-link {
  margin-right: auto;
  color: var(--c-primary);
}

.alert-link:hover {
  background: var(--c-surface-2);
}

.alert-btn {
  border: 1px solid var(--c-border);
  cursor: pointer;
}

.alert-btn:disabled {
  cursor: not-allowed;
  opacity: 0.65;
}

.alert-btn--secondary {
  background: var(--c-surface-2);
  color: var(--c-text);
}

.alert-btn--primary {
  border-color: var(--c-primary);
  background: var(--c-primary);
  color: #fff;
}

@media (max-width: 560px) {
  .alert-card__actions {
    align-items: stretch;
    flex-direction: column;
  }

  .alert-link,
  .alert-btn {
    width: 100%;
    justify-content: center;
  }
}
</style>
