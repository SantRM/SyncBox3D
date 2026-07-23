<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import AppFooter from '@/components/AppFooter.vue'
import AppNavbar from '@/components/AppNavbar.vue'
import BaseButton from '@/components/BaseButton.vue'
import BaseInput from '@/components/BaseInput.vue'
import { api } from '@/services/api'
import type { LocaleCode } from '@/services/types'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const { t } = useI18n()

const oldPwd = ref('')
const newPwd = ref('')
const confirmPwd = ref('')
const msg = ref<string | null>(null)
const err = ref<string | null>(null)
const busy = ref(false)

const languageBusy = ref(false)
const languageMsg = ref<string | null>(null)
const languageErr = ref<string | null>(null)
const currentLanguage = computed(() => auth.user?.idioma_preferido ?? 'es')

async function submit(): Promise<void> {
  msg.value = null
  err.value = null
  if (newPwd.value.length < 10) {
    err.value = t('validation.passwordMin')
    return
  }
  if (newPwd.value !== confirmPwd.value) {
    err.value = t('validation.passwordMismatch')
    return
  }
  if (oldPwd.value === newPwd.value) {
    err.value = t('validation.passwordSame')
    return
  }
  busy.value = true
  try {
    await api.auth.changePassword(oldPwd.value, newPwd.value)
    msg.value = t('auth.password.updated')
    oldPwd.value = ''
    newPwd.value = ''
    confirmPwd.value = ''
  } catch (e) {
    err.value = (e as { message?: string }).message ?? t('auth.password.error')
  } finally {
    busy.value = false
  }
}

async function changeLanguage(event: Event): Promise<void> {
  const idioma = (event.target as HTMLSelectElement).value as LocaleCode
  languageBusy.value = true
  languageMsg.value = null
  languageErr.value = null
  try {
    await auth.setLanguage(idioma)
    languageMsg.value = t('profile.languageSaved')
  } catch (e) {
    languageErr.value = (e as { message?: string }).message ?? t('profile.languageError')
  } finally {
    languageBusy.value = false
  }
}
</script>

<template>
  <div class="app-shell">
    <AppNavbar />
    <main class="page">
      <section class="perfil">
        <h1>{{ $t('profile.title') }}</h1>

        <div class="card">
          <h3>{{ $t('profile.info') }}</h3>
          <dl class="def">
            <dt>{{ $t('common.name') }}</dt><dd>{{ auth.user?.nombre }}</dd>
            <dt>{{ $t('auth.login.email') }}</dt><dd>{{ auth.user?.correo }}</dd>
            <dt>{{ $t('dashboard.role') }}</dt>
            <dd>{{ auth.user?.rol ? $t(`roles.${auth.user.rol}`) : '' }}</dd>
          </dl>
        </div>

        <div class="card">
          <h3>{{ $t('profile.preferences') }}</h3>
          <label class="select">
            <span>{{ $t('profile.language') }}</span>
            <select :value="currentLanguage" :disabled="languageBusy" @change="changeLanguage">
              <option value="es">{{ $t('profile.spanish') }}</option>
              <option value="en">{{ $t('profile.english') }}</option>
            </select>
          </label>
          <p v-if="languageErr" class="err">{{ languageErr }}</p>
          <p v-if="languageMsg" class="ok">{{ languageMsg }}</p>
        </div>

        <div class="card">
          <h3>{{ $t('auth.password.change') }}</h3>
          <form class="form" @submit.prevent="submit">
            <BaseInput v-model="oldPwd" type="password" :label="$t('auth.password.current')" :placeholder="$t('auth.password.currentPlaceholder')" autocomplete="current-password" required />
            <BaseInput v-model="newPwd" type="password" :label="$t('auth.password.next')" :placeholder="$t('auth.password.nextPlaceholder')" minlength="10" autocomplete="new-password" required />
            <BaseInput v-model="confirmPwd" type="password" :label="$t('auth.password.confirm')" :placeholder="$t('auth.password.confirmPlaceholder')" minlength="10" autocomplete="new-password" required />
            <p v-if="err" class="err">{{ err }}</p>
            <p v-if="msg" class="ok">{{ msg }}</p>
            <BaseButton type="submit" :loading="busy">{{ $t('common.update') }}</BaseButton>
          </form>
        </div>
      </section>
    </main>
    <AppFooter />
  </div>
</template>

<style scoped>
.perfil { max-width: 600px; }
.card { background: var(--c-surface); border: 1px solid var(--c-border); border-radius: var(--radius-md); padding: 1rem 1.25rem; margin-bottom: 1rem; }
.card h3 { margin-top: 0; }
.def { display: grid; grid-template-columns: 100px 1fr; gap: 0.4rem 1rem; margin: 0; }
.def dt { color: var(--c-text-muted); }
.def dd { margin: 0; }
.form { display: flex; flex-direction: column; gap: 0.75rem; }
.select { display: flex; flex-direction: column; gap: 0.35rem; font-size: 0.875rem; font-weight: 600; }
.select select { width: 100%; min-width: 0; box-sizing: border-box; padding: 0.55rem 0.5rem; border: 1px solid var(--c-border); border-radius: var(--radius-md); background: var(--c-surface); color: var(--c-text); font: inherit; }
.err { color: var(--c-danger); }
.ok { color: #16a34a; }
</style>
