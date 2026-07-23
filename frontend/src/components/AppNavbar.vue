<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRouter } from 'vue-router'

import RoleGate from '@/components/RoleGate.vue'
import logoSyncbox from '@/assets/logo-syncbox.png'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const router = useRouter()
const { t } = useI18n()
const logoSrc = logoSyncbox

const userLabel = computed(() => auth.user?.nombre ?? '')
const roleLabel = computed(() => auth.user?.rol ? t(`roles.${auth.user.rol}`) : '')

const open = ref(false)
function toggle(): void { open.value = !open.value }
function close(): void { open.value = false }

async function doLogout(): Promise<void> {
  await auth.logout()
  void router.push({ name: 'login' })
}

// Cierre automático por inactividad. Se monta una sola vez por navegación
// (cada vista monta el Navbar de nuevo, lo cual reinicia el temporizador).
const minutes = Number(import.meta.env.VITE_INACTIVITY_MIN ?? '30') || 30
const idleMs = Math.max(1, minutes) * 60_000
let idleTimer: number | null = null

function resetIdle(): void {
  if (idleTimer !== null) window.clearTimeout(idleTimer)
  idleTimer = window.setTimeout(() => void doLogout(), idleMs)
}
const events: Array<keyof WindowEventMap> = ['mousemove', 'keydown', 'click', 'touchstart']

onMounted(() => {
  for (const ev of events) window.addEventListener(ev, resetIdle, { passive: true })
  resetIdle()
})
onBeforeUnmount(() => {
  if (idleTimer !== null) window.clearTimeout(idleTimer)
  for (const ev of events) window.removeEventListener(ev, resetIdle)
})
</script>

<template>
  <header class="navbar">
    <div class="navbar__inner">
      <RouterLink class="brand" :to="{ name: 'dashboard' }" @click="close">
        <img class="brand__logo" :src="logoSrc" :alt="$t('app.name')" />
      </RouterLink>

      <button
        class="burger"
        type="button"
        :aria-expanded="open"
        aria-controls="navbar-menu"
        :aria-label="$t('nav.openMenu')"
        @click="toggle"
      >
        <span /><span /><span />
      </button>

      <nav id="navbar-menu" class="nav" :class="{ 'nav--open': open }" @click="close">
        <RouterLink :to="{ name: 'dashboard' }">{{ $t('nav.home') }}</RouterLink>
        <RoleGate :roles="['ADMINISTRADOR']">
          <RouterLink :to="{ name: 'ubicaciones' }">{{ $t('nav.locations') }}</RouterLink>
        </RoleGate>
        <RoleGate :roles="['ADMINISTRADOR']">
          <RouterLink :to="{ name: 'usuarios' }">{{ $t('nav.users') }}</RouterLink>
        </RoleGate>
        <RoleGate :roles="['ADMINISTRADOR']">
          <RouterLink :to="{ name: 'categorias' }">{{ $t('nav.categories') }}</RouterLink>
        </RoleGate>
        <RoleGate :roles="['ADMINISTRADOR', 'OPERADOR']">
          <RouterLink :to="{ name: 'alertas' }">{{ $t('nav.alerts') }}</RouterLink>
        </RoleGate>
        <RouterLink :to="{ name: 'perfil' }">{{ $t('nav.account') }}</RouterLink>
      </nav>

      <div class="user">
        <div class="user__meta">
          <div class="user__name">{{ userLabel }}</div>
          <div class="user__role">{{ roleLabel }}</div>
        </div>
        <button class="user__logout" type="button" @click="doLogout">{{ $t('nav.logout') }}</button>
      </div>
    </div>
  </header>
</template>

<style scoped>
.navbar {
  position: sticky; top: 0; z-index: 50;
  background: var(--c-surface);
  border-bottom: 1px solid var(--c-border);
}
.navbar__inner {
  max-width: 1280px;
  margin: 0 auto;
  padding: 0.6rem 1.25rem;
  display: flex; align-items: center; gap: 1.5rem;
}
.brand {
  display: inline-flex; align-items: center;
  text-decoration: none;
  flex: 0 0 auto;
}
.brand__logo {
  display: block;
  width: clamp(118px, 14vw, 168px);
  height: auto;
  max-height: 36px;
  object-fit: contain;
  background: #fff;
  border-radius: 4px;
}
.nav { display: flex; gap: 0.25rem; flex: 1; }
.nav a {
  padding: 0.5rem 0.8rem;
  border-radius: var(--radius-sm);
  text-decoration: none;
  color: var(--c-text);
  font-weight: 500;
  font-size: 0.92rem;
}
.nav a:hover { background: var(--c-surface-2); }
.nav a.router-link-active { background: var(--c-primary); color: #fff; }
.user { display: flex; align-items: center; gap: 0.75rem; }
.user__meta { text-align: right; line-height: 1.1; }
.user__name { font-weight: 600; font-size: 0.92rem; }
.user__role { font-size: 0.75rem; color: var(--c-text-muted); }
.user__logout {
  padding: 0.45rem 0.8rem; cursor: pointer;
  background: var(--c-surface-2); color: var(--c-text);
  border: 1px solid var(--c-border); border-radius: var(--radius-sm);
  font: inherit; font-size: 0.88rem;
}
.user__logout:hover { background: var(--c-surface-3); }

.burger { display: none; background: none; border: 0; padding: 0.4rem; cursor: pointer; }
.burger span { display: block; width: 22px; height: 2px; background: var(--c-text); margin: 4px 0; border-radius: 2px; }

@media (max-width: 820px) {
  .navbar__inner { flex-wrap: wrap; gap: 0.5rem; }
  .burger { display: inline-block; order: 2; margin-left: auto; }
  .user { order: 3; }
  .nav {
    order: 4;
    flex-basis: 100%;
    flex-direction: column;
    gap: 0.15rem;
    display: none;
    padding-top: 0.5rem;
    border-top: 1px solid var(--c-border);
  }
  .nav--open { display: flex; }
}
</style>
