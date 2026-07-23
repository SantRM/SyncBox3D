<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'

import BaseButton from '@/components/BaseButton.vue'
import BaseInput from '@/components/BaseInput.vue'
import logoSyncbox from '@/assets/logo-syncbox.png'
import { useAuthStore } from '@/stores/auth'

const auth = useAuthStore()
const route = useRoute()
const router = useRouter()
const { t } = useI18n()
const logoSrc = logoSyncbox

const correo = ref('')
const password = ref('')
const formError = ref<string | null>(null)

async function submit(): Promise<void> {
  formError.value = null
  try {
    await auth.login(correo.value, password.value)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    void router.push(redirect)
  } catch {
    // Mensaje neutral; no revelar si el correo existe.
    formError.value = t('auth.login.invalid')
  }
}
</script>

<template>
  <div class="login">
    <form class="login__card" @submit.prevent="submit">
      <h1 class="login__brand">
        <img class="login__logo" :src="logoSrc" :alt="$t('app.name')" />
      </h1>
      <p class="login__sub">{{ $t('auth.login.subtitle') }}</p>

      <BaseInput
        v-model="correo"
        :label="$t('auth.login.email')"
        type="email"
        :placeholder="$t('auth.login.emailPlaceholder')"
        autocomplete="username"
        maxlength="180"
        required
      />
      <BaseInput
        v-model="password"
        :label="$t('auth.login.password')"
        type="password"
        :placeholder="$t('auth.login.passwordPlaceholder')"
        autocomplete="current-password"
        required
      />

      <p v-if="formError" class="login__error" role="alert">{{ formError }}</p>

      <BaseButton type="submit" :loading="auth.loading">{{ $t('auth.login.submit') }}</BaseButton>
    </form>
  </div>
</template>

<style scoped>
.login {
  min-height: 100vh;
  display: grid; place-items: center;
  background: var(--c-bg);
  padding: 1rem;
}
.login__card {
  width: min(400px, 100%);
  background: var(--c-surface);
  border: 1px solid var(--c-border);
  border-radius: var(--radius-lg);
  padding: 1.75rem;
  display: flex; flex-direction: column; gap: 0.9rem;
  box-shadow: 0 8px 30px rgba(0,0,0,0.08);
}
.login__brand {
  margin: 0;
  display: flex;
  justify-content: center;
}
.login__logo {
  display: block;
  width: min(260px, 100%);
  height: auto;
  object-fit: contain;
  background: #fff;
  border-radius: 4px;
}
.login__sub { margin: 0 0 0.5rem; color: var(--c-text-muted); }
.login__error {
  margin: 0;
  background: #fee2e2;
  color: #7f1d1d;
  padding: 0.55rem 0.75rem;
  border-radius: var(--radius-sm);
  font-size: 0.9rem;
}
</style>
