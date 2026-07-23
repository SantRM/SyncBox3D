<script setup lang="ts">
import { ArrowRight } from '@lucide/vue'
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import AppFooter from '@/components/AppFooter.vue'
import AppNavbar from '@/components/AppNavbar.vue'
import RoleGate from '@/components/RoleGate.vue'
import { api } from '@/services/api'
import type { AlertaEvento, Equipo } from '@/services/types'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const { t } = useI18n()
const equiposCount = ref<number | null>(null)
const alertas = ref<AlertaEvento[]>([])
const loadError = ref<string | null>(null)

onMounted(async () => {
  try {
    const [eq, al] = await Promise.allSettled([
      api.equipos.list({ limit: 200 }),
      auth.hasRole('ADMINISTRADOR') ? api.alertas.pendientes() : Promise.resolve([] as AlertaEvento[]),
    ])
    if (eq.status === 'fulfilled') equiposCount.value = (eq.value as Equipo[]).length
    if (al.status === 'fulfilled') alertas.value = al.value
  } catch (e) {
    loadError.value = (e as { message?: string }).message ?? t('dashboard.loadError')
  }
})
</script>

<template>
  <div class="app-shell">
    <AppNavbar />
    <main class="page">
  <section class="dash">
    <h1>{{ $t('dashboard.welcome') }}, {{ auth.user?.nombre }}</h1>
    <p class="dash__role">{{ $t('dashboard.role') }}: {{ auth.user?.rol ? $t(`roles.${auth.user.rol}`) : '' }}</p>

    <div class="dash__cards">
      <div class="card">
        <h3>{{ $t('dashboard.equipment') }}</h3>
        <p class="card__num">{{ equiposCount ?? '—' }}</p>
        <RouterLink :to="{ name: 'ubicaciones' }">
          {{ $t('dashboard.manageLocations') }} <ArrowRight :size="14" aria-hidden="true" />
        </RouterLink>
      </div>
      <RoleGate :roles="['ADMINISTRADOR']">
        <div class="card">
          <h3>{{ $t('dashboard.pendingAlerts') }}</h3>
          <p class="card__num">{{ alertas.length }}</p>
          <RouterLink :to="{ name: 'alertas' }">
            {{ $t('dashboard.reviewAlerts') }} <ArrowRight :size="14" aria-hidden="true" />
          </RouterLink>
        </div>
      </RoleGate>
    </div>

    <p v-if="loadError" class="dash__err">{{ loadError }}</p>
  </section>
    </main>
    <AppFooter />
  </div>
</template>

<style scoped>
.dash h1 { margin: 0 0 0.25rem; }
.dash__role { color: var(--c-text-muted); margin-top: 0; }
.dash__cards { display: grid; gap: 1rem; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); margin-top: 1rem; }
.card { background: var(--c-surface); border: 1px solid var(--c-border); border-radius: var(--radius-lg); padding: 1rem 1.25rem; }
.card h3 { margin: 0 0 0.5rem; font-size: 1rem; color: var(--c-text-muted); }
.card__num { font-size: 2rem; font-weight: 700; margin: 0 0 0.5rem; }
.dash__err { margin-top: 1rem; color: var(--c-danger); }
</style>
